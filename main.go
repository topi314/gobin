package main

import (
	"embed"
	"flag"
	"html/template"
	"io"
	"log"
	"net/http"
)

var (
	//go:embed templates
	templates embed.FS

	//go:embed assets
	assets embed.FS
)

type ExecuteTemplateFunc func(wr io.Writer, name string, data any) error

type Server struct {
	cfg  Config
	db   *Database
	tmpl ExecuteTemplateFunc
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

	db, err := NewDatabase(cfg)
	if err != nil {
		log.Fatalln("Error while connecting to database:", err)
	}
	defer db.Close()

	var tmplFunc ExecuteTemplateFunc
	if cfg.DevMode {
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
		cfg:  cfg,
		db:   db,
		tmpl: tmplFunc,
	}

	log.Println("Gobin listening on:", cfg.ListenAddr)
	if err = http.ListenAndServe(cfg.ListenAddr, s.Routes()); err != nil {
		log.Fatalln("Error while listening:", err)
	}
}
