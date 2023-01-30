package main

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/go-github/v50/github"
)

func main() {
	println("Downloading latest version...")

	client := github.NewClient(nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tags, _, err := client.Repositories.ListTags(ctx, "highlightjs", "cdn-release", nil)
	if err != nil {
		println("Error: ", err.Error())
		return
	}

	if len(tags) == 0 {
		println("Error: No tags found")
		return
	}
	latestTag := tags[0]
	fmt.Printf("Latest version: %s", latestTag.GetName())

	rs, err := http.Get(latestTag.GetTarballURL())
	if err != nil {
		println("Error: ", err.Error())
		return
	}
	defer rs.Body.Close()

	data, err := io.ReadAll(rs.Body)
	if err != nil {
		println("Error: ", err.Error())
		return
	}

	files := []string{
		"build/highlight.min.js",
		"build/styles/atom-one-dark.min.css",
		"build/styles/atom-one-light.min.css",
		"build/styles/github-dark.min.css",
		"build/styles/github.min.css",
	}

	zipReader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return
	}

	for _, file := range files {
		println("Copying: ", file)
		if err = copyToAssets(zipReader, file); err != nil {
			println("Error: ", err.Error())
			return
		}
	}

	println("Done")
}

func copyToAssets(zipReader *zip.Reader, filename string) error {
	zipFile, err := zipReader.Open(filename)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	assetsFile, err := os.Open("assets/" + strings.TrimPrefix(filename, "build/"))
	if err != nil {
		return err
	}
	defer assetsFile.Close()

	_, err = io.Copy(assetsFile, zipFile)
	return err
}
