package app

const (
	version = "v1.10.11"
)

func (a *comm) GetVersion() string {
	return version
}
