package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/yeniklas/larm02/internal/config"
	"github.com/yeniklas/larm02/internal/ui"
	"github.com/yeniklas/larm02/internal/updater"
)

var version = "dev"

func main() {
	cfgPath := flag.String("config", "", "path to config file (default: ~/.config/larm02/config.yaml)")
	versionFlag := flag.Bool("version", false, "print version and exit")
	updateFlag := flag.Bool("self-update", false, "update larm02 to the latest release")
	flag.Parse()

	if *versionFlag {
		fmt.Println(version)
		os.Exit(0)
	}

	if *updateFlag {
		if err := updater.Run(version); err != nil {
			fmt.Fprintln(os.Stderr, "update:", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "larm02: %v\n", err)
		os.Exit(1)
	}

	model := ui.New(cfg)
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "larm02: %v\n", err)
		os.Exit(1)
	}
}
