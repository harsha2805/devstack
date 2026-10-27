package dns

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/sys/windows/registry"

	"github.com/harsha2805/devstack/internal/winproc"
)

// RuleComment tags devstack's NRPT rule, so it can be found and removed
// without touching rules from anything else.
const RuleComment = "devstack"

const policyKey = `SYSTEM\CurrentControlSet\Services\Dnscache\Parameters\DnsPolicyConfig`

var tldRe = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// RuleInstalled reports whether Windows sends *.tld lookups to devstack's
// responder. Reading the rules needs no administrator rights.
func RuleInstalled(tld string) bool {
	root, err := registry.OpenKey(registry.LOCAL_MACHINE, policyKey, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return false
	}
	defer root.Close()
	names, err := root.ReadSubKeyNames(-1)
	if err != nil {
		return false
	}
	server := strings.Split(Addr, ":")[0]
	for _, n := range names {
		k, err := registry.OpenKey(root, n, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		comment, _, _ := k.GetStringValue("Comment")
		spaces, _, _ := k.GetStringsValue("Name")
		servers, _, _ := k.GetStringValue("GenericDNSServers")
		k.Close()
		if comment != RuleComment || !strings.Contains(servers, server) {
			continue
		}
		for _, sp := range spaces {
			if strings.EqualFold(sp, "."+tld) {
				return true
			}
		}
	}
	return false
}

// SetRule adds (on) or removes devstack's rule for *.tld, with one
// administrator prompt. Adding replaces any earlier devstack rule, so a
// changed tld doesn't leave the old one behind.
func SetRule(on bool, tld string) error {
	if !tldRe.MatchString(tld) {
		return fmt.Errorf("%q is not a valid top-level domain", tld)
	}
	script := "$ErrorActionPreference = 'Stop'; " +
		"Get-DnsClientNrptRule | Where-Object Comment -eq '" + RuleComment + "' | Remove-DnsClientNrptRule -Force; "
	if on {
		script += "Add-DnsClientNrptRule -Namespace '." + tld + "' -NameServers '" + strings.Split(Addr, ":")[0] +
			"' -Comment '" + RuleComment + "'; "
	}
	script += "Clear-DnsClientCache"
	ps := filepath.Join(os.Getenv("SystemRoot"), `System32\WindowsPowerShell\v1.0\powershell.exe`)
	return winproc.RunElevated(ps, `-NoProfile -NonInteractive -WindowStyle Hidden -Command "`+script+`"`)
}
