package main

import (
	"context"
	"embed"
	"flag"
	"html/template"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/go-jose/go-jose/v3"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"

	"github.com/topisenpai/gobin/gobin"
)

// These variables are set via the -ldflags option in go build
var (
	version   = "unknown"
	commit    = "unknown"
	buildTime = "unknown"
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
	log.Printf("Gobin version: %s (commit: %s, build time: %s)", version, commit, buildTime)

	cfgPath := flag.String("config", "", "path to gobin.json")
	flag.Parse()

	viper.SetConfigName("gobin")
	viper.SetConfigType("json")
	if *cfgPath != "" {
		viper.SetConfigFile(*cfgPath)
	}
	viper.AddConfigPath(".")
	viper.AddConfigPath("/etc/gobin/")
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalln("Error while reading config:", err)
	}
	log.Printf("Gobin starting... (config path:%s)", *cfgPath)

	var cfg gobin.Config
	if err := viper.Unmarshal(&cfg, func(config *mapstructure.DecoderConfig) {
		config.TagName = "cfg"
	}); err != nil {
		log.Fatalln("Error while unmarshalling config:", err)
	}
	log.Println("Config:", cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := gobin.NewDB(ctx, cfg.Database, Schema)
	if err != nil {
		log.Fatalln("Error while connecting to database:", err)
	}
	defer db.Close()

	key := jose.SigningKey{
		Algorithm: jose.HS512,
		Key:       []byte(cfg.JWTSecret),
	}
	signer, err := jose.NewSigner(key, nil)
	if err != nil {
		log.Fatalln("Error while creating signer:", err)
	}

	var (
		tmplFunc gobin.ExecuteTemplateFunc
		assets   http.FileSystem
	)
	if cfg.DevMode {
		log.Println("Development mode enabled")
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
			log.Fatalln("Error while parsing templates:", err)
		}
		tmplFunc = tmpl.ExecuteTemplate
		assets = http.FS(Assets)
	}

	s := gobin.NewServer(gobin.FormatVersion(version, commit, buildTime), cfg, db, signer, assets, tmplFunc)
	log.Println("Gobin listening on:", cfg.ListenAddr)
	s.Start()
}
