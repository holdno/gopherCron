package main

import (
	"os"

	"github.com/spf13/cobra"
	"ojbk.io/gopherCron/cmd/client"
	"ojbk.io/gopherCron/cmd/service"
)

func main() {
	root := &cobra.Command{
		Use: "gophercron",
	}
	root.AddCommand(
		service.NewCommand(),
		client.NewCommand())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
