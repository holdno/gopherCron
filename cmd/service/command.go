package service

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// SetupOptions init api options
type SetupOptions struct {
	LogLevel   string
	ConfigPath string
	ProxyOnly  bool
}

// New a function to return a inited SetupOptions
func newSetupOptions() *SetupOptions {
	return &SetupOptions{}
}

// AddFlags related to api options
func (o *SetupOptions) AddFlags(flagSet *pflag.FlagSet) {
	// Add flags for generic options
	flagSet.StringVarP(&o.LogLevel, "log-level", "l", "INFO", "log print level")
	flagSet.StringVarP(&o.ConfigPath, "config", "c", "./cmd/service/conf/config-dev.toml", "init api by given config")
	flagSet.BoolVarP(&o.ProxyOnly, "proxyonly", "p", false, "setup proxy only")
}

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
