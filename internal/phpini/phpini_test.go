package phpini

import (
	"slices"
	"strings"
	"testing"
)

const sample = `[PHP]
; For example:
;   extension=mysqli
memory_limit = 128M
memory_limit = 256M
;max_input_vars = 1000
;opcache.enable_cli=0
;extension=curl
extension=php_gd.dll
;extension=exif      ; Must be after mbstring
;zend_extension=opcache
`

func TestSet(t *testing.T) {
	got := Set(sample, "memory_limit", "1G")
	if !strings.Contains(got, "memory_limit = 128M\nmemory_limit = 1G\n") {
		t.Errorf("the last active line should change:\n%s", got)
	}
	if got := Set(sample, "max_input_vars", "5000"); !strings.Contains(got, "\nmax_input_vars = 5000\n") || strings.Contains(got, ";max_input_vars") {
		t.Errorf("a commented-out line should be used:\n%s", got)
	}
	if got := Set(sample, "opcache.enable", "1"); !strings.HasSuffix(got, ";zend_extension=opcache\nopcache.enable = 1\n") {
		t.Errorf("opcache.enable_cli isn't opcache.enable; append instead:\n%s", got)
	}
}

func TestExtensions(t *testing.T) {
	if got := Extensions(sample); !slices.Equal(got, []string{"gd"}) {
		t.Errorf("Extensions = %v", got)
	}
	got := Enable(sample, "exif", false)
	if !strings.Contains(got, "\nextension=exif      ; Must be after mbstring\n") {
		t.Errorf("exif not uncommented:\n%s", got)
	}
	if got := Enable(sample, "mysqli", false); !strings.Contains(got, ";   extension=mysqli") || !strings.Contains(got, ";extension=exif      ; Must be after mbstring\nextension=mysqli\n") {
		t.Errorf("mysqli should be added after the last extension line, not taken from the prose:\n%s", got)
	}
	if got := Enable(sample, "opcache", true); !strings.Contains(got, "\nzend_extension=opcache\n") {
		t.Errorf("opcache not uncommented:\n%s", got)
	}
	if got := Disable(sample, func(n string) bool { return n == "gd" }); len(Extensions(got)) != 0 || !strings.Contains(got, ";extension=php_gd.dll") {
		t.Errorf("gd not commented out:\n%s", got)
	}
	if got := AddOff(sample, "sqlsrv"); !strings.Contains(got, "\n;extension=sqlsrv\n") || !Listed(got, "sqlsrv") || slices.Contains(Extensions(got), "sqlsrv") {
		t.Errorf("sqlsrv should be listed, off:\n%s", got)
	}
}

func TestFull(t *testing.T) {
	if !Full(sample) || Full("; mine\r\nmemory_limit = 2G\r\n") {
		t.Error("Full")
	}
	if got := ExtName(`"C:\php\ext\php_sqlsrv_83_nts_x64.dll"`); got != "sqlsrv_83_nts_x64" {
		t.Errorf("ExtName = %s", got)
	}
}

func TestUnset(t *testing.T) {
	got := Unset(sample, "memory_limit")
	if !strings.Contains(got, ";memory_limit = 128M\n;memory_limit = 256M\n") {
		t.Errorf("every active line should be commented out:\n%s", got)
	}
	if s := Settings(got); len(s) != 0 {
		t.Errorf("Settings after Unset = %v", s)
	}
	if got := Set(got, "memory_limit", "1G"); !strings.Contains(got, "memory_limit = 1G\n;memory_limit = 256M") {
		t.Errorf("Set after Unset should reuse the first line:\n%s", got)
	}
}

func TestExtensionStates(t *testing.T) {
	want := []Ext{{"curl", false}, {"gd", true}, {"exif", false}}
	if got := ExtensionStates(sample); !slices.Equal(got, want) {
		t.Errorf("ExtensionStates = %v, want %v", got, want)
	}
	if got := ExtensionStates(sample + "extension=curl\n"); !got[0].On {
		t.Errorf("curl listed off and on should be on: %v", got)
	}
}
