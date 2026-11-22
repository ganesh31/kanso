package main

import "testing"

func TestListenAddrExpandsEnv(t *testing.T) {
	t.Setenv("PORT", "9090")
	for in, want := range map[string]string{
		":${PORT}":       ":9090",
		":$PORT":         ":9090",
		"127.0.0.1:8080": "127.0.0.1:8080",
	} {
		if got := listenAddr(in); got != want {
			t.Errorf("listenAddr(%q) = %q, want %q", in, got, want)
		}
	}
}
