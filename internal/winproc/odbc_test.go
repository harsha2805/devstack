package winproc

import (
	"slices"
	"testing"
)

func TestPHPODBCDriver(t *testing.T) {
	d17, d18, d13 := "ODBC Driver 17 for SQL Server", "ODBC Driver 18 for SQL Server", "ODBC Driver 13 for SQL Server"
	for _, c := range []struct {
		installed []string
		want      string
	}{
		{[]string{d18, d17}, d17},
		{[]string{d18}, d18},
		{[]string{d13, d18}, d18},
		{[]string{d13}, d13},
		{nil, ""},
	} {
		if got, ok := PHPODBCDriver(c.installed); got != c.want || ok != (c.want != "") {
			t.Errorf("%v: got %q, %v; want %q", c.installed, got, ok, c.want)
		}
	}

	names := []string{d13, d17, d18}
	sortODBC(names)
	if !slices.Equal(names, []string{d18, d17, d13}) {
		t.Errorf("sorted %v", names)
	}
}
