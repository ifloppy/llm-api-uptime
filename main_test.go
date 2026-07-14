package main

import "testing"

func TestServerMode(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{name: "server", args: []string{"--server"}, want: true},
		{name: "default", args: nil, want: false},
		{name: "version", args: []string{"--version"}, want: false},
		{name: "extra argument", args: []string{"--server", "extra"}, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := serverMode(test.args); got != test.want {
				t.Fatalf("serverMode(%q) = %v, want %v", test.args, got, test.want)
			}
		})
	}
}
