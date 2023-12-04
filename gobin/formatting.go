package gobin

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/topi314/gobin/gobin/database"
)

func getFormatter(r *http.Request, fallback bool) chroma.Formatter {
	formatterName := r.URL.Query().Get("formatter")
	if formatterName == "" {
		if !fallback {
			return formatters.NoOp
		}
		formatterName = "html"
	}

	formatter := formatters.Get(formatterName)
	if formatter == nil {
		return formatters.Fallback
	}

	return formatter
}

func (s *Server) formatFile(file database.File, formatter chroma.Formatter, style *chroma.Style) (string, error) {
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
