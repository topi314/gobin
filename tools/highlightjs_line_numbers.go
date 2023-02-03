package main

import (
	"fmt"
	"log"

	"github.com/google/go-github/v50/github"
)

const highlightJSLineNumbersFiles = "dist/highlightjs-line-numbers.min.js"

func downloadHighlightJSLineNumbers(client *github.Client) error {
	zipReader, err := downloadLatestTagZipball(client, "wcoder", "highlightjs-line-numbers.js")
	if err != nil {
		return fmt.Errorf("failed to download zip: %w", err)
	}

	prefix := zipReader.File[0].Name

	log.Println("Copying: ", prefix+highlightJSLineNumbersFiles)
	if err = copyToAssets(zipReader, prefix, "dist/", highlightJSLineNumbersFiles); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return nil
}
