package main

import (
	"log"

	"github.com/trevorashby/llamasitter/internal/cli"
)

func main() {
	if err := cli.GenerateReferenceDocs("docs/reference/cli"); err != nil {
		log.Fatal(err)
	}
}
