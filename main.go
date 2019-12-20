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
				world, err := OpenAnvilWorld(c.Args().Get(0))
				if err != nil {
					return err
				}

				testFile, err := os.OpenFile("/home/andrew/go/src/github.com/astei/anvil2slime/test.slime", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
				if err != nil {
					return err
				}
				err = world.WriteAsSlime(testFile)
				if err != nil {
					return err
				}
				err = testFile.Close()
				if err != nil {
					return err
				}
			}
			return nil
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
