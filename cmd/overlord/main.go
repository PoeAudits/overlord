package main

import (
	"os"

	"github.com/PoeAudits/overlord/internal/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
