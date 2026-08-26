package main

import "testing"

func TestResolveServerURLPriority(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		args []string
		env  string
		want string
	}{
		{name: "flag wins", args: []string{"--server", "https://flag.example"}, env: "https://env.example", want: "https://flag.example"},
		{name: "environment fallback", env: "https://env.example", want: "https://env.example"},
		{name: "local default", want: "http://localhost:8080"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveServerURL(tc.args, func(key string) string {
				if key == "TERMCHAT_SERVER_URL" {
					return tc.env
				}
				return ""
			})
			if err != nil {
				t.Fatalf("resolveServerURL() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("resolveServerURL() = %q, want %q", got, tc.want)
			}
		})
	}
}
