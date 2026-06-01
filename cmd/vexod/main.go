package main

import (
	"os"

	"github.com/vexo-network/vexo-consensus/config"
)

func main() {
	writeStatus(os.Stdout, config.Default("vexo-local"))
}
