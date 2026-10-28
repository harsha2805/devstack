package winproc

import (
	"regexp"
	"slices"
	"strconv"
)

var sqlServerODBC = regexp.MustCompile(`^ODBC Driver (\d+) for SQL Server$`)

// phpODBCOrder is the order PHP's sqlsrv and pdo_sqlsrv try the ODBC drivers
// in when the connection string names none: 17 before 18, so an upgrade to
// 18, which encrypts by default, doesn't change existing connections.
// Checked against pdo_sqlsrv 5.13 with 17 and 18 both installed.
var phpODBCOrder = []int{17, 18, 13}

// ODBCSQLServer returns the "ODBC Driver N for SQL Server" that PHP's sqlsrv
// and pdo_sqlsrv connect through when the connection string doesn't name one.
func ODBCSQLServer() (string, bool) {
	return PHPODBCDriver(ODBCSQLServerDrivers())
}

// PHPODBCDriver picks the driver PHP uses from the installed ones.
func PHPODBCDriver(installed []string) (string, bool) {
	for _, want := range phpODBCOrder {
		for _, n := range installed {
			if odbcVersion(n) == want {
				return n, true
			}
		}
	}
	return "", false
}

func odbcVersion(name string) int {
	m := sqlServerODBC.FindStringSubmatch(name)
	if m == nil {
		return 0
	}
	v, _ := strconv.Atoi(m[1])
	return v
}

// sortODBC orders driver names newest first.
func sortODBC(names []string) {
	slices.SortFunc(names, func(a, b string) int { return odbcVersion(b) - odbcVersion(a) })
}
