package version

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/holdno/gopherCron/protocol"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "echo current system version",
		Run: func(c *cobra.Command, args []string) {
			fmt.Println(protocol.GetVersion())
		},
	}
	return cmd
}
