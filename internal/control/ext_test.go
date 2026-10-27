package control

import "testing"

func TestVendorName(t *testing.T) {
	for e, want := range map[string]bool{
		"sqlsrv_83_nts_x64": true, "sqlsrv_84_ts_x86": true, "sqlsrv": false, "sqlsrvx_83_nts_x64": false,
	} {
		if got := vendorName(e, "sqlsrv"); got != want {
			t.Errorf("vendorName(%q, sqlsrv) = %v", e, got)
		}
	}
	// pdo_sqlsrv's vendor name must not count as sqlsrv's.
	if vendorName("pdo_sqlsrv_83_nts_x64", "sqlsrv") {
		t.Error("pdo_sqlsrv_83_nts_x64 matched sqlsrv")
	}
	if !vendorName("pdo_sqlsrv_83_nts_x64", "pdo_sqlsrv") {
		t.Error("pdo_sqlsrv_83_nts_x64 didn't match pdo_sqlsrv")
	}
}
