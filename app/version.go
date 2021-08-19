package app

const (
	version = "v1.10.12"
)

func (a *comm) GetVersion() string {
	return version
}
