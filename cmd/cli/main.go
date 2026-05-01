package main

import (
	"log"

	"github.com/xboard/xboard/internal/cmd"
	_ "github.com/xboard/xboard/internal/cmd/commands" // import all commands
)

func main() {
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
