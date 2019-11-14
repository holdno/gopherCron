package client

import "github.com/spf13/cobra"

// NewCommand of client
func NewCommand() *cobra.Command {
	opt := newSetupOptions()
	cmd := &cobra.Command{
		Use:   "client",
		Short: "job agent",
		RunE: func(c *cobra.Command, args []string) error {
			if err := Run(opt); err != nil {
				return err
			}
			return nil
		},
	}
	opt.AddFlags(cmd.Flags())
	return cmd
}
