package client

import "testing"

func FailIf(err error, t *testing.T) {
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
}
