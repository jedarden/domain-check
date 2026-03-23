package main

import (
	"fmt"
	"os"

	"github.com/coding/domain-check/internal/config"
)

func main() {
	cfg, err := config.Load(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Config loaded: %+v\n", cfg)
}
