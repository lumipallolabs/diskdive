package logging

import (
	"io"
	"log"
	"os"
)

var (
	Debug   *log.Logger
	Scanner *log.Logger
	Enabled bool
)

func init() {
	// Only enable logging if DISKDIVE_DEBUG environment variable is set
	if os.Getenv("DISKDIVE_DEBUG") == "" {
		// Create no-op loggers that discard output
		Debug = log.New(io.Discard, "", 0)
		Scanner = log.New(io.Discard, "", 0)
		Enabled = false
		return
	}

	Enabled = true

	// Open debug.log once for all loggers
	debugFile, err := os.OpenFile("debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		// Fallback to stderr if we can't open the file
		Debug = log.New(os.Stderr, "[DEBUG] ", log.Ldate|log.Ltime)
		Scanner = log.New(os.Stderr, "[SCANNER] ", log.Ldate|log.Ltime)
		return
	}

	// Create loggers with different prefixes sharing the same file
	Debug = log.New(debugFile, "", log.Lmicroseconds)
	Scanner = log.New(debugFile, "", log.Lmicroseconds)
}
