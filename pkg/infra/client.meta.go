package infra

import (
	"net/url"
	"path/filepath"
	"strconv"
)

// resolver service meta
type ResolveMeta struct {
	OrgID  string
	Region string
	System int64
}

func (r ResolveMeta) FullServiceName(srvName string) string {
	target := filepath.ToSlash(filepath.Join(r.OrgID, strconv.FormatInt(r.System, 10), srvName))
	path, _ := url.Parse(target)
	if r.Region != "" {
		query := path.Query()
		query.Add("region", r.Region)
		path.RawQuery = query.Encode()
	}
	return path.String()
}
