package gobin

import (
	"io"
	"log"
	"net/http"

	"github.com/go-jose/go-jose/v3"
)

type ExecuteTemplateFunc func(wr io.Writer, name string, data any) error

func NewServer(cfg Config, db *DB, signer jose.Signer, assets http.FileSystem, tmpl ExecuteTemplateFunc) *Server {
	return &Server{
		cfg:    cfg,
		db:     db,
		signer: signer,
		assets: assets,
		tmpl:   tmpl,
	}
}

type Server struct {
	cfg    Config
	db     *DB
	signer jose.Signer
	assets http.FileSystem
	tmpl   ExecuteTemplateFunc
}

func (s *Server) Start() {
	if err := http.ListenAndServe(s.cfg.ListenAddr, s.Routes()); err != nil {
		log.Fatalln("Error while listening:", err)
	}
}
