package buildinfo

import "testing"

func TestCurrent(t *testing.T) {
	want := Info{Version: Version, Commit: Commit, BuildDate: BuildDate}
	if got := Current(); got != want {
		t.Fatalf("Current() = %#v, want %#v", got, want)
	}
}
