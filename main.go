package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/pprof"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lumipallolabs/diskdive/internal/ui/tui"
)

func main() {
	// Enable CPU profiling if CPUPROFILE env var is set
	if cpuProfile := os.Getenv("CPUPROFILE"); cpuProfile != "" {
		f, err := os.Create(cpuProfile)
		if err != nil {
			log.Fatal("could not create CPU profile: ", err)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("could not start CPU profile: ", err)
		}
		defer pprof.StopCPUProfile()
		log.Printf("CPU profiling enabled, writing to %s", cpuProfile)
	}

	// Check for path argument
	var scanPath string
	if len(os.Args) > 1 {
		path := os.Args[1]
		absPath, err := filepath.Abs(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid path: %v\n", err)
			os.Exit(1)
		}
		scanPath = absPath
	}

	p := tea.NewProgram(
		tui.NewApp(Version, scanPath),
		tea.WithAltScreen(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
