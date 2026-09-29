package api

import "testing"

func TestHandle(t *testing.T) {
	if Handle("a") != "v:a" {
		t.Fatal("handle")
	}
}
