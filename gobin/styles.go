package gobin

type Style struct {
	Name string
	URL  string
}

var Styles = []Style{
	{Name: "Atom One Dark", URL: "atom-one-dark.min.css"},
	{Name: "Atom One Light", URL: "atom-one-light.min.css"},
	{Name: "GitHub Dark", URL: "github-dark.min.css"},
	{Name: "GitHub Light", URL: "github.min.css"},
}
