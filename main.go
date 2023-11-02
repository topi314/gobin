package main

import (
	"context"
	"embed"
	"flag"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/go-jose/go-jose/v3"
	"github.com/lmittmann/tint"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
	"github.com/topi314/gobin/gobin"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// These variables are set via the -ldflags option in go build
var (
	Name      = "gobin"
	Namespace = "github.com/topi314/gobin"

	Version   = "unknown"
	Commit    = "unknown"
	BuildTime = "unknown"
)

var (
	//go:embed templates
	Templates embed.FS

	//go:embed assets
	Assets embed.FS

	//go:embed sql/schema.sql
	Schema string
)

func main() {
	cfgPath := flag.String("config", "", "path to gobin.json")
	flag.Parse()

	viper.SetDefault("log_level", "info")
	viper.SetDefault("log_format", "json")
	viper.SetDefault("log_add_source", false)
	viper.SetDefault("listen_addr", ":80")
	viper.SetDefault("debug", false)
	viper.SetDefault("dev_mode", false)
	viper.SetDefault("database_type", "sqlite")
	viper.SetDefault("database_debug", false)
	viper.SetDefault("database_expire_after", "0")
	viper.SetDefault("database_cleanup_interval", "1m")
	viper.SetDefault("database_path", "gobin.db")
	viper.SetDefault("database_host", "localhost")
	viper.SetDefault("database_port", 5432)
	viper.SetDefault("database_username", "gobin")
	viper.SetDefault("database_database", "gobin")
	viper.SetDefault("database_ssl_mode", "disable")
	viper.SetDefault("max_document_size", 0)

	if *cfgPath != "" {
		viper.SetConfigFile(*cfgPath)
	} else {
		viper.SetConfigName("gobin")
		viper.SetConfigType("json")
		viper.AddConfigPath(".")
		viper.AddConfigPath("/etc/gobin/")
	}
	if err := viper.ReadInConfig(); err != nil {
		slog.Error("Error while reading config", tint.Err(err))
		os.Exit(1)
	}
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.SetEnvPrefix("gobin")
	viper.AutomaticEnv()

	var cfg gobin.Config
	if err := viper.Unmarshal(&cfg, func(config *mapstructure.DecoderConfig) {
		config.TagName = "cfg"
	}); err != nil {
		slog.Error("Error while unmarshalling config", tint.Err(err))
		os.Exit(1)
	}

	setupLogger(cfg.Log)
	buildTime, _ := time.Parse(time.RFC3339, BuildTime)
	slog.Info("Starting Gobin...", slog.String("version", Version), slog.String("commit", Commit), slog.Time("build-time", buildTime))
	slog.Info("Config", slog.String("config", cfg.String()))

	var (
		tracer trace.Tracer
		meter  metric.Meter
		err    error
	)
	if cfg.Otel != nil {
		tracer, err = newTracer(*cfg.Otel)
		if err != nil {
			slog.Error("Error while creating tracer", tint.Err(err))
			os.Exit(1)
		}
		meter, err = newMeter(*cfg.Otel)
		if err != nil {
			slog.Error("Error while creating meter", tint.Err(err))
			os.Exit(1)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := gobin.NewDB(ctx, cfg.Database, Schema)
	if err != nil {
		slog.Error("Error while connecting to database", tint.Err(err))
		os.Exit(1)
	}
	defer db.Close()

	signer, err := jose.NewSigner(jose.SigningKey{
		Algorithm: jose.HS512,
		Key:       []byte(cfg.JWTSecret),
	}, nil)
	if err != nil {
		slog.Error("Error while creating signer", tint.Err(err))
		os.Exit(1)
	}

	var (
		tmplFunc gobin.ExecuteTemplateFunc
		assets   http.FileSystem
	)
	if cfg.DevMode {
		slog.Info("Development mode enabled")
		tmplFunc = func(wr io.Writer, name string, data any) error {
			tmpl, err := template.New("").ParseGlob("templates/*")
			if err != nil {
				return err
			}
			return tmpl.ExecuteTemplate(wr, name, data)
		}
		assets = http.Dir(".")
	} else {
		tmpl, err := template.New("").ParseFS(Templates, "templates/*")
		if err != nil {
			slog.Error("Error while parsing templates", tint.Err(err))
			os.Exit(1)
		}
		tmplFunc = tmpl.ExecuteTemplate
		assets = http.FS(Assets)
	}

	styles.Fallback = styles.Get("onedark")
	lexers.Fallback = lexers.Get("plaintext")
	formatters.Register("html", html.New(
		html.WithClasses(true),
		html.ClassPrefix("ch-"),
		html.Standalone(false),
		html.InlineCode(false),
		html.WithNopPreWrapper(),
		html.WithLineNumbers(true),
		html.WithLinkableLineNumbers(true, "L"),
		html.TabWidth(4),
	))
	formatters.Register("html-standalone", html.New(
		html.Standalone(true),
		html.WithLineNumbers(true),
		html.WithLinkableLineNumbers(true, "L"),
		html.TabWidth(4),
	))

	s := gobin.NewServer(gobin.FormatBuildVersion(Version, Commit, buildTime), cfg.DevMode, cfg, db, signer, tracer, meter, assets, tmplFunc)
	slog.Info("Gobin started...", slog.String("address", cfg.ListenAddr))
	go s.Start()
	defer s.Close()

	si := make(chan os.Signal, 1)
	signal.Notify(si, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-si
}

func setupLogger(cfg gobin.LogConfig) {
	var handler slog.Handler
	if cfg.Format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			AddSource: cfg.AddSource,
			Level:     cfg.Level,
		})
	} else {
		handler = tint.NewHandler(os.Stdout, &tint.Options{
			Level:     cfg.Level,
			AddSource: cfg.AddSource,
		})
	}
	slog.SetDefault(slog.New(handler))
}
