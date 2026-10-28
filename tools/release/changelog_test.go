package main

import "testing"

func TestParseChangelog(t *testing.T) {
	md := "# Changelog\r\n\r\nMaintainer notes.\r\n\r\n## [0.2.3] — unreleased\r\n\r\n### Added\r\n- **Shims.** php per site.\r\n\r\n## [0.2.2] — 2026-10-28\r\n\r\n### Fixed\r\n- **ODBC.** The right driver.\r\n"
	c := parseChangelog(md)
	if len(c.Versions) != 2 {
		t.Fatalf("versions: %+v", c.Versions)
	}
	if v := c.Versions[0]; v.Version != "0.2.3" || v.Date != "" || v.Markdown != "### Added\n- **Shims.** php per site." {
		t.Errorf("first: %+v", v)
	}
	if v := c.Versions[1]; v.Version != "0.2.2" || v.Date != "2026-10-28" || v.Markdown != "### Fixed\n- **ODBC.** The right driver." {
		t.Errorf("second: %+v", v)
	}
}
