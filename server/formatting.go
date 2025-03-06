package server

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/topi314/chroma/v2"
	"github.com/topi314/chroma/v2/formatters"
	"github.com/topi314/chroma/v2/lexers"

	"github.com/topi314/gobin/v3/server/database"
)

func getFormatter(r *http.Request, fallback bool) (chroma.Formatter, string) {
	formatterName := r.URL.Query().Get("formatter")
	if formatterName == "" {
		if !fallback {
			return nil, ""
		}
		formatterName = "html"
	}

	formatter := formatters.Get(formatterName)
	if formatter == nil {
		return formatters.Fallback, ""
	}

	return formatter, formatterName
}

func (s *Server) formatFile(file database.File, formatter chroma.Formatter, style *chroma.Style) (string, error) {
	if formatter == nil {
		return file.Content, nil
	}
	lexer := lexers.Get(file.Language)
	if s.cfg.MaxHighlightSize > 0 && len([]rune(file.Content)) > s.cfg.MaxHighlightSize {
		lexer = lexers.Get("plaintext")
	}
	if lexer == nil {
		lexer = lexers.Fallback
	}

	iterator, err := lexer.Tokenise(nil, file.Content)
	if err != nil {
		return "", fmt.Errorf("tokenise: %w", err)
	}

	buff := new(bytes.Buffer)
	if err = formatter.Format(buff, style, iterator); err != nil {
		return "", fmt.Errorf("format: %w", err)
	}

	return buff.String(), nil
}
