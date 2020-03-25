package main

import (
	"os"

	"github.com/holdno/gopherCron/cmd/client"
	"github.com/holdno/gopherCron/cmd/service"
	"github.com/spf13/cobra"
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
