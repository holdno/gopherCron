package client

import "github.com/spf13/pflag"

// SetupOptions init api options
type SetupOptions struct {
	LogLevel      string
	ConfigPath    string
	ReportAddress string
}

// New a function to return a inited SetupOptions
func newSetupOptions() *SetupOptions {
	return &SetupOptions{}
}

// AddFlags related to api options
func (o *SetupOptions) AddFlags(flagSet *pflag.FlagSet) {
	// Add flags for generic options
	flagSet.StringVarP(&o.LogLevel, "log-level", "l", "INFO", "log print level")
	flagSet.StringVarP(&o.ConfigPath, "config", "c", "./client/conf/config-test.toml", "init client by given config")
	flagSet.StringVarP(&o.ReportAddress, "reportaddr", "r", "", "report task log to log service")
}
