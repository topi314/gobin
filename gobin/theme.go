package gobin

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"

	"github.com/topi314/chroma/v2"
	"github.com/topi314/chroma/v2/styles"
)

func getStyle(r *http.Request) *chroma.Style {
	var styleName string
	if styleCookie, err := r.Cookie("style"); err == nil {
		styleName = styleCookie.Value
	}
	queryStyle := r.URL.Query().Get("style")
	if queryStyle != "" {
		styleName = queryStyle
	}

	style := styles.Get(styleName)
	if style == nil {
		return styles.Fallback
	}

	return style
}

func (s *Server) ThemeCSS(w http.ResponseWriter, r *http.Request) {
	style := getStyle(r)
	cssBuff := s.themeCSS(style)

	w.Header().Set("Content-Type", "text/css; charset=UTF-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(cssBuff)))
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write([]byte(cssBuff))
}

func (s *Server) themeCSS(style *chroma.Style) string {
	cssBuff := new(bytes.Buffer)
	background := style.Get(chroma.Background)
	_, _ = fmt.Fprintf(cssBuff, "html{color-scheme: %s;}", style.Theme)
	_, _ = fmt.Fprint(cssBuff, ":root{")
	_, _ = fmt.Fprintf(cssBuff, "--bg-primary: %s;", background.Background.String())
	_, _ = fmt.Fprintf(cssBuff, "--bg-secondary: %s;", background.Background.BrightenOrDarken(0.07).String())
	_, _ = fmt.Fprintf(cssBuff, "--nav-button-bg: %s;", background.Background.BrightenOrDarken(0.12).String())
	_, _ = fmt.Fprintf(cssBuff, "--text-primary: %s;", background.Colour.String())
	_, _ = fmt.Fprintf(cssBuff, "--text-secondary: %s;", background.Colour.BrightenOrDarken(0.2).String())
	_, _ = fmt.Fprintf(cssBuff, "--bg-scrollbar: %s;", background.Background.BrightenOrDarken(0.1).String())
	_, _ = fmt.Fprintf(cssBuff, "--bg-scrollbar-thumb: %s;", background.Background.BrightenOrDarken(0.2).String())
	_, _ = fmt.Fprintf(cssBuff, "--bg-scrollbar-thumb-hover: %s;", background.Background.BrightenOrDarken(0.3).String())
	_, _ = fmt.Fprint(cssBuff, "}")

	_ = s.htmlFormatter.WriteCSS(cssBuff, style)
	return cssBuff.String()
}
