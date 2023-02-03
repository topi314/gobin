package main

import (
	"fmt"
	"log"

	"github.com/google/go-github/v50/github"
)

var highlightJSFiles = []string{
	"build/highlight.min.js",
	"build/styles/atom-one-dark.min.css",
	"build/styles/atom-one-light.min.css",
	"build/styles/github-dark.min.css",
	"build/styles/github.min.css",
}

func downloadHighlightJS(client *github.Client) error {
	zipReader, err := downloadLatestTagZipball(client, "highlightjs", "cdn-release")
	if err != nil {
		return fmt.Errorf("failed to download zip: %w", err)
	}

	prefix := zipReader.File[0].Name
	for _, file := range highlightJSFiles {
		log.Println("Copying: ", prefix+file)
		if err = copyToAssets(zipReader, prefix, "build/", file); err != nil {
			return fmt.Errorf("failed to copy file: %w", err)
		}
	}

	return nil
}
