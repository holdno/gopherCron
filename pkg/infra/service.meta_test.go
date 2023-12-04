package infra

import (
	"testing"

	"github.com/spacegrower/watermelon/infra/register"
	"google.golang.org/grpc/attributes"
)

func TestMetaIsComarable(t *testing.T) {
	type attributeKey struct{}
	var a *attributes.Attributes
	var b *attributes.Attributes

	a = attributes.New(attributeKey{}, NodeMeta{
		OrgID:               "123",
		CenterServiceRegion: "test",
		NodeMeta: register.NodeMeta{
			Host: "localhost",
			Port: 6666,
		},
	})

	b = attributes.New(attributeKey{}, NodeMeta{
		OrgID:               "123",
		CenterServiceRegion: "test",
		NodeMeta: register.NodeMeta{
			Host: "localhost",
			Port: 6666,
		},
	})

	t.Log(a.Equal(b))
}
