package utils

import "testing"

func TestGetDateFromNow(t *testing.T) {
	t.Log(GetDateFromNow(-0).Format("2006-01-02"))
}
