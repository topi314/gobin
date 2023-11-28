package main

import (
	"context"
	"embed"
	"flag"
	"io/fs"
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
	"github.com/mattn/go-colorable"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
	"github.com/topi314/gobin/gobin"
	"github.com/topi314/gobin/gobin/database"
	"github.com/topi314/tint"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/topi314/gobin/gobin"
)

//go:generate go run github.com/a-h/templ/cmd/templ@latest generate

// These variables are set via the -ldflags option in go build
var (
	Name      = "gobin"
	Namespace = "github.com/topi314/gobin"

	Version   = "unknown"
	Commit    = "unknown"
	BuildTime = "unknown"
)

var (
	//go:embed assets
	Assets embed.FS

	//go:embed sql/schema.sql
	Schema string

	//go:embed styles
	Styles embed.FS
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
	viper.SetDefault("default_style", "onedark")

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
	db, err := database.New(ctx, cfg.Database, Schema)
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

	var assets http.FileSystem
	if cfg.DevMode {
		slog.Info("Development mode enabled")
		assets = http.Dir(".")
	} else {
		assets = http.FS(Assets)
	}

	loadEmbeddedStyles()
	loadLocalStyles(cfg.CustomStyles)

	styles.Fallback = styles.Get(cfg.DefaultStyle)
	lexers.Fallback = lexers.Get("plaintext")
	htmlFormatter := html.New(
		html.WithClasses(true),
		html.ClassPrefix("ch-"),
		html.Standalone(false),
		html.InlineCode(false),
		html.WithNopPreWrapper(),
		html.WithLineNumbers(true),
		html.WithLinkableLineNumbers(true, "L"),
		html.TabWidth(4),
	)
	formatters.Register("html", htmlFormatter)
	formatters.Register("html-standalone", html.New(
		html.Standalone(true),
		html.WithLineNumbers(true),
		html.WithLinkableLineNumbers(true, "L"),
		html.TabWidth(4),
	))

	s := gobin.NewServer(gobin.FormatBuildVersion(Version, Commit, buildTime), cfg.DevMode, cfg, db, signer, tracer, meter, assets, htmlFormatter)
	slog.Info("Gobin started...", slog.String("address", cfg.ListenAddr))
	go s.Start()
	defer s.Close()

	si := make(chan os.Signal, 1)
	signal.Notify(si, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-si
}

const (
	ansiFaint         = "\033[2m"
	ansiWhiteBold     = "\033[37;1m"
	ansiYellowBold    = "\033[33;1m"
	ansiCyanBold      = "\033[36;1m"
	ansiCyanBoldFaint = "\033[36;1;2m"
	ansiRedFaint      = "\033[31;2m"
	ansiRedBold       = "\033[31;1m"

	ansiRed     = "\033[31m"
	ansiYellow  = "\033[33m"
	ansiGreen   = "\033[32m"
	ansiMagenta = "\033[35m"
)

func setupLogger(cfg gobin.LogConfig) {
	var handler slog.Handler
	switch cfg.Format {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			AddSource: cfg.AddSource,
			Level:     cfg.Level,
		})

	case "text":
		handler = tint.NewHandler(colorable.NewColorable(os.Stdout), &tint.Options{
			AddSource: cfg.AddSource,
			Level:     cfg.Level,
			NoColor:   cfg.NoColor,
			LevelColors: map[slog.Level]string{
				slog.LevelDebug: ansiMagenta,
				slog.LevelInfo:  ansiGreen,
				slog.LevelWarn:  ansiYellow,
				slog.LevelError: ansiRed,
			},
			Colors: map[tint.Kind]string{
				tint.KindTime:            ansiYellowBold,
				tint.KindSourceFile:      ansiCyanBold,
				tint.KindSourceSeparator: ansiCyanBoldFaint,
				tint.KindSourceLine:      ansiCyanBold,
				tint.KindMessage:         ansiWhiteBold,
				tint.KindKey:             ansiFaint,
				tint.KindSeparator:       ansiFaint,
				tint.KindValue:           ansiWhiteBold,
				tint.KindErrorKey:        ansiRedFaint,
				tint.KindErrorSeparator:  ansiFaint,
				tint.KindErrorValue:      ansiRedBold,
			},
		})
	default:
		slog.Error("Unknown log format", slog.String("format", cfg.Format))
		os.Exit(-1)
	}
	slog.SetDefault(slog.New(handler))
}

func loadEmbeddedStyles() {
	slog.Info("Loading embedded styles")
	stylesSub, err := fs.Sub(Styles, "styles")
	if err != nil {
		slog.Error("Failed to get sub fs for embedded styles", tint.Err(err))
		return
	}
	cStyles, err := styles.LoadFromFS(stylesSub)
	if err != nil {
		slog.Error("Failed to load embedded styles", tint.Err(err))
		return
	}
	for _, style := range cStyles {
		slog.Debug("Loaded embedded style", slog.String("name", style.Name))
		styles.Register(style)
	}
}

func loadLocalStyles(stylesDir string) {
	if stylesDir == "" {
		return
	}

	slog.Info("Loading local styles", slog.String("dir", stylesDir))
	cStyles, err := styles.LoadFromFS(os.DirFS(stylesDir))
	if err != nil {
		slog.Error("Failed to load local styles", tint.Err(err))
		return
	}
	for _, style := range cStyles {
		slog.Debug("Loaded local style", slog.String("name", style.Name))
		styles.Register(style)
	}
}
