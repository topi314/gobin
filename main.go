package main

import (
	"context"
	"embed"
	"flag"
	"golang.org/x/exp/slog"
	"html/template"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/go-jose/go-jose/v3"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"

	"github.com/topisenpai/gobin/gobin"
)

// These variables are set via the -ldflags option in go build
var (
	Name      = "gobin"
	Namespace = "github.com/topisenpai/gobin"

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

var levels = map[string]slog.Level{
	"debug": slog.LevelDebug,
	"info":  slog.LevelInfo,
	"warn":  slog.LevelWarn,
	"error": slog.LevelError,
}

func main() {
	cfgPath := flag.String("config", "", "path to gobin.json")
	debug := flag.Bool("debug", false, "debug mode")
	logType := flag.String("log", "json", "log format, one of: json, text")
	logLevel := flag.String("log-level", "info", "log level, one of: debug, info, warn, error")
	flag.Parse()

	setupLogger(*debug, *logType, *logLevel)

	buildTime, _ := time.Parse(time.RFC3339, BuildTime)
	slog.Info("Starting Gobin...", slog.String("version", Version), slog.String("commit", Commit), slog.Time("build-time", buildTime))

	viper.SetDefault("listen_addr", ":80")
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
		slog.Error("Error while reading config", slog.Any("err", err))
		os.Exit(1)
	}
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.SetEnvPrefix("gobin")
	viper.AutomaticEnv()

	var cfg gobin.Config
	if err := viper.Unmarshal(&cfg, func(config *mapstructure.DecoderConfig) {
		config.TagName = "cfg"
	}); err != nil {
		slog.Error("Error while unmarshalling config", slog.Any("err", err))
	}
	slog.Info("Config", slog.String("config", cfg.String()))

	var (
		tracer trace.Tracer
		meter  metric.Meter
		err    error
	)
	if cfg.Otel != nil {
		tracer, err = newTracer(*cfg.Otel)
		if err != nil {
			slog.Error("Error while creating tracer", slog.Any("err", err))
			os.Exit(1)
		}
		meter, err = newMeter(*cfg.Otel)
		if err != nil {
			slog.Error("Error while creating meter", slog.Any("err", err))
			os.Exit(1)
		}
	}

	db, err := gobin.NewDB(context.Background(), cfg.Database, Schema)
	if err != nil {
		slog.Error("Error while connecting to database", slog.Any("err", err))
		os.Exit(1)
	}
	defer db.Close()

	signer, err := jose.NewSigner(jose.SigningKey{
		Algorithm: jose.HS512,
		Key:       []byte(cfg.JWTSecret),
	}, nil)
	if err != nil {
		slog.Error("Error while creating signer", slog.Any("err", err))
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
			slog.Error("Error while parsing templates", slog.Any("err", err))
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

	s := gobin.NewServer(gobin.FormatBuildVersion(Version, Commit, buildTime), *debug, cfg, db, signer, tracer, meter, assets, tmplFunc)
	slog.Info("Gobin started...", slog.String("address", cfg.ListenAddr))
	go s.Start()
	defer s.Close()

	si := make(chan os.Signal, 1)
	signal.Notify(si, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-si
}

func setupLogger(debug bool, level string, logType string) {
	handlerOpts := slog.HandlerOptions{
		AddSource: debug,
		Level:     levels[level],
	}
	var handler slog.Handler
	if logType == "json" {
		handler = handlerOpts.NewJSONHandler(os.Stdout)
	} else {
		handler = handlerOpts.NewTextHandler(os.Stdout)
	}
	slog.SetDefault(slog.New(handler))
}
