package main

import (
	"os"

	"github.com/common-fate/clio"
	"github.com/common-fate/clio/clierr"
	"github.com/common-fate/docs/cmd/docscli/commands/generate"
	"github.com/urfave/cli/v3"
)

func main() {
	app := &cli.App{
		Name:        "docs",
		Version:     "v0.0.1",
		HideVersion: false,
		Commands:    []*cli.Command{&generate.GenerateCommand},
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "verbose", Usage: "Enable verbose logging"},
		},
		Before: func(ctx *cli.Context) error {
			if ctx.Bool("verbose") {
				clio.SetLevelFromString("debug")
			}
			return nil
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		// if the error is an instance of clio.PrintCLIErrorer then print the error accordingly
		if cliError, ok := err.(clierr.PrintCLIErrorer); ok {
			cliError.PrintCLIError()
		} else {
			clio.Error(err.Error())
		}
		os.Exit(1)
	}
}
