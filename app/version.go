package app

const (
	version = "v1.10.5"
)

func (a *comm) GetVersion() string {
	return version
}
