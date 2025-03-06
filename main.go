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
	"syscall"
	"time"

	"github.com/charmbracelet/log"
	"github.com/go-jose/go-jose/v3"
	"github.com/topi314/chroma/v2/formatters"
	"github.com/topi314/chroma/v2/formatters/html"
	"github.com/topi314/chroma/v2/lexers"
	"github.com/topi314/chroma/v2/styles"

	"github.com/topi314/gobin/v3/internal/ver"
	"github.com/topi314/gobin/v3/server"
	"github.com/topi314/gobin/v3/server/database"
)

//go:generate go run github.com/a-h/templ/cmd/templ@latest generate

var (
	//go:embed server/assets
	Assets embed.FS

	//go:embed server/migrations/*
	Migrations embed.FS

	//go:embed styles
	Styles embed.FS
)

func main() {
	cfgPath := flag.String("config", "gobin.toml", "path to gobin.toml")
	flag.Parse()

	cfg, err := server.LoadConfig(*cfgPath)
	if err != nil {
		slog.Error("Error while loading config", slog.Any("err", err))
		return
	}

	setupLogger(cfg.Log)
	version := ver.Load()
	slog.Info("Starting Gobin...", slog.String("version", version.Version), slog.String("commit", version.Revision), slog.String("build-time", version.BuildTime))
	slog.Info("Config", slog.String("config", cfg.String()))

	if err = server.SetupOtel(version.Version, cfg.Otel); err != nil {
		slog.Error("Error while setting up otel", slog.Any("err", err))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := database.New(ctx, cfg.Database, Migrations)
	if err != nil {
		slog.Error("Error while connecting to database", slog.Any("err", err))
		return
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			slog.Error("Error while closing database", slog.Any("err", closeErr))
		}
	}()

	signer, err := jose.NewSigner(jose.SigningKey{
		Algorithm: jose.HS512,
		Key:       []byte(cfg.JWTSecret),
	}, nil)
	if err != nil {
		slog.Error("Error while creating signer", slog.Any("err", err))
		return
	}

	var assets http.FileSystem
	if cfg.DevMode {
		slog.Info("Development mode enabled")
		assets = http.Dir("server")
	} else {
		sub, err := fs.Sub(Assets, "server")
		if err != nil {
			slog.Error("Failed to get sub fs for embedded assets", slog.Any("err", err))
			return
		}
		assets = http.FS(sub)
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
	standaloneHTMLFormatter := html.New(
		html.Standalone(true),
		html.WithLineNumbers(true),
		html.WithLinkableLineNumbers(true, "L"),
		html.TabWidth(4),
	)
	formatters.Register("html", htmlFormatter)
	formatters.Register("html-standalone", standaloneHTMLFormatter)

	s := server.NewServer(version, cfg.DevMode, cfg, db, signer, assets, htmlFormatter, standaloneHTMLFormatter)
	slog.Info("Gobin started...", slog.String("address", cfg.ListenAddr))
	go s.Start()
	defer s.Close()

	si := make(chan os.Signal, 1)
	signal.Notify(si, syscall.SIGINT, syscall.SIGTERM)
	<-si
}

func setupLogger(cfg server.LogConfig) {
	var handler slog.Handler
	switch cfg.Format {
	case server.LogFormatJSON:
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			AddSource: cfg.AddSource,
			Level:     cfg.Level,
		})

	case server.LogFormatText:
		handler = log.NewWithOptions(os.Stdout, log.Options{
			Level:        log.Level(cfg.Level),
			ReportCaller: cfg.AddSource,
		})
	default:
		slog.Error("Unknown log format", slog.String("format", string(cfg.Format)))
		os.Exit(-1)
	}
	slog.SetDefault(slog.New(handler))
}

func loadEmbeddedStyles() {
	slog.Info("Loading embedded styles")
	stylesSub, err := fs.Sub(Styles, "styles")
	if err != nil {
		slog.Error("Failed to get sub fs for embedded styles", slog.Any("err", err))
		return
	}
	cStyles, err := styles.LoadFromFS(stylesSub)
	if err != nil {
		slog.Error("Failed to load embedded styles", slog.Any("err", err))
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
		slog.Error("Failed to load local styles", slog.Any("err", err))
		return
	}
	for _, style := range cStyles {
		slog.Debug("Loaded local style", slog.String("name", style.Name))
		styles.Register(style)
	}
}
