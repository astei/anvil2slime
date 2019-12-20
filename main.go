package main

import (
	"fmt"
	"github.com/urfave/cli/v2"
	"log"
	"os"
	"path/filepath"
	"time"
)

func main() {
	app := &cli.App{
		Name:  "anvil2slime",
		Usage: "converts Anvil worlds to Slime and back",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Usage:   "writes the Slime region to the specified `FILE`",
			},
		},
		Action: func(c *cli.Context) error {
			if c.NArg() == 0 {
				_, _ = fmt.Fprintf(os.Stderr, "need a world to work with!\n")
				return nil
			} else {
				return processAnvilWorld(c.Args().Get(0), c.String("output"))
			}
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func processAnvilWorld(path string, saveTo string) (err error) {
	startAnvilLoad := time.Now()
	world, err := OpenAnvilWorld(filepath.Join(path, "region"))
	if err != nil {
		return err
	}
	loadAnvilDuration := time.Now().Sub(startAnvilLoad).Milliseconds()
	fmt.Printf("Anvil world loaded in %dms\n", loadAnvilDuration)

	if saveTo == "" {
		saveTo = filepath.Join(filepath.Dir(path), filepath.Base(path)+".slime")
	}
	outputFile, err := os.OpenFile(saveTo, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	startSlimeSave := time.Now()
	if err = world.WriteAsSlime(outputFile); err != nil {
		return err
	}
	slimeSaveDuration := time.Now().Sub(startSlimeSave).Milliseconds()
	fmt.Printf("Slime world saved in %dms\n", slimeSaveDuration)

	err = outputFile.Close()
	return
}
