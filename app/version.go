package app

const (
	version = "v1.10.10"
)

func (a *comm) GetVersion() string {
	return version
}
