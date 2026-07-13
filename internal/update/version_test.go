package update

import "testing"

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name      string
		current   string
		latest    string
		want      int
		wantError bool
	}{
		{name: "older patch", current: "v1.2.3", latest: "v1.2.4", want: -1},
		{name: "newer major", current: "2.0.0", latest: "v1.99.99", want: 1},
		{name: "same ignoring metadata", current: "v1.2.3+local", latest: "1.2.3+release", want: 0},
		{name: "large component", current: "1.999999999999999999999.0", latest: "1.1000000000000000000000.0", want: -1},
		{name: "prerelease current", current: "1.2.3-rc.1", latest: "1.2.3", wantError: true},
		{name: "prerelease latest", current: "1.2.2", latest: "1.2.3-beta", wantError: true},
		{name: "development current", current: "dev", latest: "1.2.3", wantError: true},
		{name: "leading zero", current: "01.2.3", latest: "1.2.3", wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := CompareVersions(test.current, test.latest)
			if (err != nil) != test.wantError {
				t.Fatalf("CompareVersions() error = %v, wantError %v", err, test.wantError)
			}
			if err == nil && got != test.want {
				t.Fatalf("CompareVersions() = %d, want %d", got, test.want)
			}
		})
	}
}

func TestAssetName(t *testing.T) {
	if got := AssetName("v1.2.3", "linux", "arm64"); got != "llm-api-uptime_v1.2.3_linux_arm64" {
		t.Fatalf("AssetName() = %q", got)
	}
	if got := AssetName("v1.2.3", "windows", "amd64"); got != "llm-api-uptime_v1.2.3_windows_amd64.exe" {
		t.Fatalf("AssetName() = %q", got)
	}
}
