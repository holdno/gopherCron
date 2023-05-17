package infra

import (
	"net/url"
	"path/filepath"
	"strconv"

	"github.com/holdno/gopherCron/common"
	"github.com/spacegrower/watermelon/infra"
	"google.golang.org/grpc/metadata"
)

var _ infra.ClientServiceNameGenerator = (*ResolveMeta)(nil)

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

func (r ResolveMeta) ProxyMetadata() metadata.MD {
	md := metadata.New(map[string]string{})
	if r.OrgID != "" {
		md.Set("gophercron-org-id", r.OrgID)
	}
	if r.Region != "" {
		md.Set("gophercron-region", r.Region)
	}
	if r.System != 0 {
		md.Set(common.GOPHERCRON_PROXY_PROJECT_MD_KEY, strconv.FormatInt(r.System, 10))
	}
	return md
}
