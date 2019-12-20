package main

import (
	"fmt"
	"github.com/urfave/cli/v2"
	"log"
	"os"
)

func main() {
	app := &cli.App{
		Name:  "anvil2slime",
		Usage: "converts Anvil worlds to Slime and back",
		Action: func(c *cli.Context) error {
			if c.NArg() == 0 {
				_, _ = fmt.Fprintf(os.Stderr, "need a world to work with")
				return nil
			} else {
				return OpenAnvilWorld(c.Args().Get(0))
			}
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
