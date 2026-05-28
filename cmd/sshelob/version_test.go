package main

import "testing"

func TestFormatVersion(t *testing.T) {
	tests := []struct {
		name      string
		version   string
		commit    string
		buildDate string
		want      string
	}{
		{
			name:      "keeps parseable build date unchanged",
			version:   "v0.1.0",
			commit:    "abc1234",
			buildDate: "2026-05-27T10:11:12Z",
			want:      "sshelob v0.1.0 (commit abc1234, built 2026-05-27T10:11:12Z)",
		},
		{
			name:      "keeps unparseable build date",
			version:   "v0.2.0",
			commit:    "def5678",
			buildDate: "unknown",
			want:      "sshelob v0.2.0 (commit def5678, built unknown)",
		},
	}

	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			got := formatVersion(testCase.version, testCase.commit, testCase.buildDate)
			if got != testCase.want {
				t.Fatalf("format mismatch:\n got: %q\nwant: %q", got, testCase.want)
			}
		})
	}
}
