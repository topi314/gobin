package main

import (
	"archive/zip"
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/go-github/v50/github"
)

func main() {
	githubToken := flag.String("github-token", "", "GitHub token (required)")
	flag.Parse()
	println("Downloading latest version...")

	var client *github.Client
	if *githubToken != "" {
		github.NewTokenClient(context.Background(), *githubToken)
	} else {
		client = github.NewClient(nil)
	}

	if err := downloadHighlightJS(client); err != nil {
		println("Failed to download HighlightJS:", err.Error())
		return
	}

	if err := downloadHighlightJSLineNumbers(client); err != nil {
		println("Failed to download HighlightJS Line Numbers:", err.Error())
		return
	}

	println("Done")
}

func downloadLatestTagZipball(client *github.Client, owner string, repo string) (*zip.Reader, error) {
	latestTag, err := getLatestTag(client, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest tag: %w", err)
	}
	println("Latest", owner+"/"+repo, "version:", latestTag.GetName())

	println(owner+"/"+repo, "Zipball URL:", latestTag.GetZipballURL())

	zipReader, err := downloadZip(latestTag.GetZipballURL())
	if err != nil {
		return nil, fmt.Errorf("failed to download zip: %w", err)
	}

	if len(zipReader.File) == 0 {
		return nil, fmt.Errorf("zip is empty")
	}

	return zipReader, nil
}

func getLatestTag(client *github.Client, owner string, repo string) (*github.RepositoryTag, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tags, _, err := client.Repositories.ListTags(ctx, owner, repo, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}

	if len(tags) == 0 {
		return nil, fmt.Errorf("no tags found")
	}

	return tags[0], nil
}

func downloadZip(url string) (*zip.Reader, error) {
	rs, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to download zip: %w", err)
	}
	defer rs.Body.Close()

	data, err := io.ReadAll(rs.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read zip: %w", err)
	}

	zipReader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("failed to read zip: %w", err)
	}

	if len(zipReader.File) == 0 {
		return nil, fmt.Errorf("zip is empty")
	}

	return zipReader, nil
}

func copyToAssets(zipReader *zip.Reader, prefix string, buildPrefix string, filename string) error {
	zipFile, err := zipReader.Open(prefix + filename)
	if err != nil {
		return fmt.Errorf("failed to open zip file: %w", err)
	}
	defer zipFile.Close()

	assetsFile, err := os.OpenFile("assets/"+strings.TrimPrefix(filename, buildPrefix), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open assets file: %w", err)
	}
	defer assetsFile.Close()

	_, err = io.Copy(assetsFile, zipFile)
	return err
}
