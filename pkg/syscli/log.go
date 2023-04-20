package syscli

type Logger interface {
	Info(string)
	Infof(string, ...interface{})
	Warn(string)
	Warnf(string, ...interface{})
	Error(string)
	Errorf(string, ...interface{})
}

var _ Logger = (*nopLogger)(nil)

type nopLogger struct{}

func (nopLogger) Info(msg string)                           {}
func (nopLogger) Infof(format string, args ...interface{})  {}
func (nopLogger) Warn(msg string)                           {}
func (nopLogger) Warnf(format string, args ...interface{})  {}
func (nopLogger) Error(msg string)                          {}
func (nopLogger) Errorf(format string, args ...interface{}) {}

func newNopLogger() nopLogger {
	return nopLogger{}
}
