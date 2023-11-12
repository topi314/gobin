package templates

import (
	"html/template"
)

type (
	Variables struct {
		ID        string
		Version   int64
		Content   template.HTML
		Formatted template.HTML
		CSS       template.CSS
		ThemeCSS  template.CSS
		Language  string

		Versions []DocumentVersion
		Lexers   []string
		Styles   []Style
		Style    string
		Theme    string

		Max        int
		Host       string
		Preview    bool
		PreviewAlt string
	}

	DocumentVersion struct {
		Version int64
		Label   string
		Time    string
	}

	Style struct {
		Name  string
		Theme string
	}

	ErrorVariables struct {
		Error     string
		Status    int
		RequestID string
		Path      string
	}
)
