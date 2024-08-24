package register

import "testing"

func TestMapRangeAndDelete(t *testing.T) {
	a := make(map[string]string)
	a["a"] = "a"
	a["b"] = "b"
	a["c"] = "c"
	a["d"] = "d"
	a["e"] = "e"
	for k, v := range a {
		t.Log(k, v)
		delete(a, k)
	}

	t.Log(a)
}
