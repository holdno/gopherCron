package selection

import (
	"testing"
)

func TestNewSelector(t *testing.T) {
	a := NewSelector(NewRequirement("name", Like, 1))
	for _, v := range a.Query {
		query, value := v.String()
		t.Log(query, value)
	}
}
