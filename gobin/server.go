package gobin

import (
	"io"
	"log"
	"net/http"
)

type ExecuteTemplateFunc func(wr io.Writer, name string, data any) error

func NewServer(cfg Config, db *DB, assets http.FileSystem, tmpl ExecuteTemplateFunc) *Server {
	return &Server{
		cfg:    cfg,
		db:     db,
		assets: assets,
		tmpl:   tmpl,
	}
}

type Server struct {
	cfg    Config
	db     *DB
	assets http.FileSystem
	tmpl   ExecuteTemplateFunc
}

func (s *Server) Start() {
	if err := http.ListenAndServe(s.cfg.ListenAddr, s.Routes()); err != nil {
		log.Fatalln("Error while listening:", err)
	}
}
