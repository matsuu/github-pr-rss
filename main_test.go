package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunEndToEnd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search/issues" {
			t.Errorf("request path = %q, want /search/issues", r.URL.Path)
		}
		if got, want := r.URL.Query().Get("q"), `repo:Homebrew/homebrew-cask is:pr is:merged label:"new cask"`; got != want {
			t.Errorf("q = %q, want %q", got, want)
		}
		if got := r.URL.Query().Get("sort"); got != "created" {
			t.Errorf("sort = %q, want created", got)
		}
		if got := r.URL.Query().Get("per_page"); got != "20" {
			t.Errorf("per_page = %q, want 20", got)
		}
		json.NewEncoder(w).Encode(sampleResult())
	}))
	defer srv.Close()

	var out bytes.Buffer
	err := run([]string{
		"-api", srv.URL,
		"-repo", "Homebrew/homebrew-cask",
		"-state", "merged",
		"-label", "new cask",
		"-limit", "20",
		"-pretty",
	}, &out, io.Discard, func(string) string { return "" })
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	s := out.String()
	if !strings.HasPrefix(s, "<?xml ") {
		t.Errorf("output does not start with an XML declaration:\n%s", s)
	}
	if !strings.Contains(s, "<rss version=\"2.0\"") {
		t.Errorf("output is not an RSS 2.0 document:\n%s", s)
	}
	if !strings.Contains(s, "<title>PR #42: Add foo 1.2.3</title>") {
		t.Errorf("output missing PR item:\n%s", s)
	}
}

func TestRunFlagErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "missing repo", args: nil, wantErr: "-repo"},
		{name: "bad repo", args: []string{"-repo", "Homebrew"}, wantErr: "-repo"},
		{name: "invalid state", args: []string{"-repo", "a/b", "-state", "bogus"}, wantErr: "-state"},
		{
			name:    "positional URL no longer accepted",
			args:    []string{"-repo", "a/b", "https://github.com/Homebrew/homebrew-cask/pulls?q=is%3Apr"},
			wantErr: "unexpected argument",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := run(tt.args, io.Discard, io.Discard, func(string) string { return "" })
			if err == nil {
				t.Fatalf("run(%q) succeeded, want error containing %q", tt.args, tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("run(%q) error = %q, want it to contain %q", tt.args, err, tt.wantErr)
			}
		})
	}
}
