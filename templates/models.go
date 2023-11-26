package templates

import (
	"context"
	"io"
	"strconv"

	"github.com/a-h/templ"
)

func WriteUnsafe(str string) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		_, err := w.Write([]byte(str))
		return err
	})
}

type DocumentVars struct {
	ID      string
	Version int64

	Files    []File
	Versions []DocumentVersion

	Lexers []string
	Styles []Style
	Style  string
	Theme  string

	Max        int
	Host       string
	Preview    bool
	PreviewAlt string
}

type File struct {
	Name             string
	Content          string
	ContentFormatted string
	Language         string
}

func (v DocumentVars) PreviewURL() string {
	url := "https://" + v.Host + "/" + v.ID
	if v.Version > 0 {
		url += "/" + strconv.FormatInt(v.Version, 10)
	}
	return url + "/preview"
}

func (v DocumentVars) URL() string {
	return "https://" + v.Host
}

type DocumentVersion struct {
	Version int64
	Label   string
	Time    string
}

type Style struct {
	Name  string
	Theme string
}

type ErrorVars struct {
	Error     string
	Status    int
	Path      string
	RequestID string
}
