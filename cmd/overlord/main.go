package main

import (
	"os"

	"github.com/PoeAudits/overlord/internal/cmd"
)

func main() {
	os.Exit(cmd.Execute())
}
