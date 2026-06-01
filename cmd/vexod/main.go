package main

import (
	"fmt"
	"os"

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
	writeStatus(os.Stdout, config.Default("vexo-local"))
}
