package gobin

import "github.com/alecthomas/chroma/v2/formatters/html"

var _ html.PreWrapper = (*NoopPreWrapper)(nil)

type NoopPreWrapper struct{}

func (n NoopPreWrapper) Start(code bool, styleAttr string) string {
	return ""
}

func (n NoopPreWrapper) End(code bool) string {
	return ""
}
