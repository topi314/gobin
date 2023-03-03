package gobin

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"runtime"
	"time"
)

type ExecuteTemplateFunc func(wr io.Writer, name string, data any) error

func NewServer(version string, cfg Config, db *DB, assets http.FileSystem, tmpl ExecuteTemplateFunc) *Server {
	return &Server{
		version: version,
		cfg:     cfg,
		db:      db,
		assets:  assets,
		tmpl:    tmpl,
	}
}

type Server struct {
	version string
	cfg     Config
	db      *DB
	assets  http.FileSystem
	tmpl    ExecuteTemplateFunc
}

func (s *Server) Start() {
	if err := http.ListenAndServe(s.cfg.ListenAddr, s.Routes()); err != nil {
		log.Fatalln("Error while listening:", err)
	}
}

func FormatVersion(version string, commit string, buildTime string) string {
	if len(commit) > 7 {
		commit = commit[:7]
	}

	buildTimeStr := "unknown"
	if buildTime != "unknown" {
		parsedTime, _ := time.Parse(time.RFC3339, buildTime)
		if !parsedTime.IsZero() {
			buildTimeStr = parsedTime.Format(time.ANSIC)
		}
	}
	return fmt.Sprintf("Go Version: %s\nVersion: %s\nCommit: %s\nBuild Time: %s\nOS/Arch: %s/%s\n", runtime.Version(), version, commit, buildTimeStr, runtime.GOOS, runtime.GOARCH)
}
