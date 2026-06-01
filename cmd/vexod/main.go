package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/vexo-network/vexo-consensus/config"
)

func main() {
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
