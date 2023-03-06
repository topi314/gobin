#!/bin/sh
# scripts/completions.sh
set -e
rm -rf completions
mkdir completions
for sh in bash zsh fish powershell; do
	go run cli/main.go completion "$sh" >"completions/gobin-cli.$sh"
done