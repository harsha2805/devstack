package main

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
)

// Changelog is CHANGELOG.md as the website reads it from each release.
type Changelog struct {
	Versions []ChangelogVersion `json:"versions"`
}

// ChangelogVersion is one "## [x.y.z] — date" section.
type ChangelogVersion struct {
	Version  string `json:"version"`
	Date     string `json:"date,omitempty"` // "2026-10-28"; empty while unreleased
	Markdown string `json:"markdown"`       // the section's body, without its heading
}

var versionHeading = regexp.MustCompile(`(?m)^## \[(\d+\.\d+\.\d+)\][^\n]*$`)
var isoDate = regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)

// parseChangelog splits markdown into its version sections, newest first as
// written. Text before the first version (notes for maintainers) is dropped.
func parseChangelog(markdown string) Changelog {
	markdown = strings.ReplaceAll(markdown, "\r\n", "\n")
	c := Changelog{Versions: []ChangelogVersion{}}
	heads := versionHeading.FindAllStringSubmatchIndex(markdown, -1)
	for i, h := range heads {
		end := len(markdown)
		if i+1 < len(heads) {
			end = heads[i+1][0]
		}
		heading := markdown[h[0]:h[1]]
		c.Versions = append(c.Versions, ChangelogVersion{
			Version:  markdown[h[2]:h[3]],
			Date:     isoDate.FindString(heading),
			Markdown: strings.TrimSpace(markdown[h[1]:end]),
		})
	}
	return c
}

func writeChangelog(in, out string) error {
	b, err := os.ReadFile(in)
	if err != nil {
		return err
	}
	j, err := json.MarshalIndent(parseChangelog(string(b)), "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(out, append(j, '\n'), 0o644)
}
