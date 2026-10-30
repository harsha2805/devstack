package control

import (
	"strings"
	"testing"
)

func TestLocationWarnings(t *testing.T) {
	t.Setenv("OneDrive", `C:\Users\me\OneDrive`)
	t.Setenv("OneDriveConsumer", "")
	t.Setenv("OneDriveCommercial", "")
	cases := map[string]string{
		`\\wsl.localhost\Ubuntu\home\me\app`: "WSL",
		`\\wsl$\Ubuntu\home\me\app`:          "WSL",
		`\\fileserver\projects\app`:          "network",
		`C:\Users\me\OneDrive\Desktop\app`:   "OneDrive",
		`C:\Users\me\OneDriveBackup\app`:     "",
		`D:\work\app`:                        "",
	}
	for dir, want := range cases {
		got := strings.Join(LocationWarnings(dir), " ")
		if want == "" && got != "" || want != "" && !strings.Contains(got, want) {
			t.Errorf("%s: got %q, want a warning mentioning %q", dir, got, want)
		}
	}
}
