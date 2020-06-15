package service

import (
	"github.com/spf13/pflag"
)

// SetupOptions init api options
type SetupOptions struct {
	LogLevel   string
	ConfigPath string
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
}
