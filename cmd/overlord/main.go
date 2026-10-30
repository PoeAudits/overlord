package main

import (
	"fmt"
	"os"

	"github.com/PoeAudits/overlord/internal/cmd"
	"github.com/PoeAudits/overlord/internal/system"
)

func main() {
	sys, err := system.OS()
	if err != nil {
		fmt.Fprintln(os.Stderr, "overlord:", err)
		os.Exit(1)
	}
	os.Exit(cmd.Execute(sys, os.Args[1:]))
}
