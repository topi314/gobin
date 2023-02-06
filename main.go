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
)

var (
	//go:embed templates
	templates embed.FS

	//go:embed assets
	assets embed.FS
)

type ExecuteTemplateFunc func(wr io.Writer, name string, data any) error

type Server struct {
	cfg    Config
	db     *Database
	signer jose.Signer
	tmpl   ExecuteTemplateFunc
}

func main() {
	cfgPath := flag.String("config", "config.json", "path to config.json")
	flag.Parse()

	log.Println("Gobin starting... (config path:", *cfgPath, ")")

	cfg, err := LoadConfig(*cfgPath)
	if err != nil {
		log.Fatalln("Error while reading config:", err)
	}
	log.Println("Config:", cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := NewDatabase(ctx, cfg)
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

	var tmplFunc ExecuteTemplateFunc
	if cfg.DevMode {
		log.Println("Development mode enabled")
		tmplFunc = func(wr io.Writer, name string, data any) error {
			tmpl, err := template.New("").ParseGlob("templates/*")
			if err != nil {
				return err
			}
			return tmpl.ExecuteTemplate(wr, name, data)
		}
	} else {
		tmpl, err := template.New("").ParseFS(templates, "templates/*")
		if err != nil {
			log.Fatalln("Error while parsing templates:", err)
		}
		tmplFunc = tmpl.ExecuteTemplate
	}

	s := &Server{
		cfg:    cfg,
		db:     db,
		signer: signer,
		tmpl:   tmplFunc,
	}

	log.Println("Gobin listening on:", cfg.ListenAddr)
	if err = http.ListenAndServe(cfg.ListenAddr, s.Routes()); err != nil {
		log.Fatalln("Error while listening:", err)
	}
}
