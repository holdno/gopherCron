package service

import "github.com/spf13/cobra"

// NewCommand of service
func NewCommand() *cobra.Command {
	opt := newSetupOptions()
	cmd := &cobra.Command{
		Use:   "service",
		Short: "A api service",
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
