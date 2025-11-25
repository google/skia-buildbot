package main

import (
	"fmt"
)

// Config holds the configuration for the conversion.
type Config struct {
	InputFile  string
	OutputFile string
}

// Run performs the JSON conversion.
func Run(cfg Config) error {
	fmt.Printf("Input file: %s\n", cfg.InputFile)
	fmt.Printf("Output file: %s\n", cfg.OutputFile)

	// TODO(eduardoyap): Implement JSON conversion logic here
	fmt.Println("Conversion logic not yet implemented.")
	return nil
}
