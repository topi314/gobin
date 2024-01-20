package main

import (
	"flag"
	"log"
	"os"

	"github.com/charmbracelet/bubbletea"
	"github.com/topi314/gobin/v2/tui/cmd"
)

func main() {
	debug := flag.Bool("debug", false, "enable debug mode")
	flag.Parse()
	if debug != nil && *debug == true {
		f, err := tea.LogToFile("debug.log", "debug")
		if err != nil {
			log.Println("fatal:", err)
			os.Exit(1)
		}
		defer f.Close()
		log.Println("debug mode enabled")
	}

	p := tea.NewProgram(cmd.NewMain(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
