package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/vexo-network/vexo-consensus/config"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "init" {
		if err := runInit(os.Stdout, os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "init failed: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "validate" {
		if err := runValidate(os.Stdout, os.Args[2:]); err != nil {
			if errors.Is(err, errValidationFailed) {
				fmt.Fprintf(os.Stderr, "%v\n", err)
			} else {
				fmt.Fprintf(os.Stderr, "validate failed: %v\n", err)
			}
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "keys" {
		if err := runKeys(os.Stdout, os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "keys failed: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "status" {
		if len(os.Args) > 2 && os.Args[2] == "--json" {
			if err := writeStatusJSON(os.Stdout, config.Default("vexo-local")); err != nil {
				fmt.Fprintf(os.Stderr, "status failed: %v\n", err)
				os.Exit(1)
			}
			return
		}
		writeStatus(os.Stdout, config.Default("vexo-local"))
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "--json" {
		if err := writeStatusJSON(os.Stdout, config.Default("vexo-local")); err != nil {
			fmt.Fprintf(os.Stderr, "status failed: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "demo" {
		if err := writeDemo(os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "demo failed: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "store-demo" {
		path := filepath.Join(os.TempDir(), "vexo-consensus-store-demo")
		if len(os.Args) > 2 {
			path = os.Args[2]
		}
		if err := writeStoreDemo(os.Stdout, path); err != nil {
			fmt.Fprintf(os.Stderr, "store demo failed: %v\n", err)
			os.Exit(1)
		}
		return
	}
	writeStatus(os.Stdout, config.Default("vexo-local"))
}
