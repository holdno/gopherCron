package app

const (
	version = "v1.10.3"
)

func (a *comm) GetVersion() string {
	return version
}
