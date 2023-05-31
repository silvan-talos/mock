package main

import (
	"errors"
	"log"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/silvan-talos/mock/mocking"
)

var (
	ErrArgs = errors.New("please provide at least interface name or file path. Run `mock --help` for details")
)

func main() {
	app := &cli.App{
		Name:  "mock",
		Usage: "generate mocks for Go interfaces",
		Authors: []*cli.Author{
			{
				Name:  "Silvan Talos",
				Email: "silvantalos@ymail.com",
			},
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "file",
				Aliases: []string{"f"},
				Usage:   "`PATH` of the file containing interface(s) to mock. If not defined, it will look in all .go files recurrently in child directories",
			},
			&cli.StringSliceFlag{
				Name:    "interface",
				Aliases: []string{"i"},
				Usage:   "`NAME` of the interface(s) to mock. If not defined will mock all the interfaces in the file provided",
			},
		},
		Action: startMocker,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func startMocker(cCtx *cli.Context) error {
	filePath := cCtx.String("file")
	interfaces := cCtx.StringSlice("interface")
	if filePath == "" && interfaces == nil {
		return ErrArgs
	}
	mocker := mocking.NewMocker()
	ms := mocking.NewService(mocker)
	err := ms.Process(interfaces, filePath)
	if err != nil {
		return err
	}
	return nil
}
