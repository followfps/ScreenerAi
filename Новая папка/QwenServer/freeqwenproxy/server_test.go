package freeqwenproxy

import "testing"

func TestNormalizeAPIVersionPath(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{in: "/api/v1/chat", want: "/api/chat"},
		{in: "/api/v2/chat/completions", want: "/api/chat/completions"},
		{in: "/api//v1//models", want: "/api/models"},
		{in: "/api/v2", want: "/api"},
		{in: "/api/status", want: "/api/status"},
	}
	for _, tc := range cases {
		if got := normalizeAPIVersionPath(tc.in); got != tc.want {
			t.Fatalf("in=%q got=%q want=%q", tc.in, got, tc.want)
		}
	}
}

