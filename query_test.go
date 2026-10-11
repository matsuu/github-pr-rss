package main

import (
	"strings"
	"testing"
)

func TestNewFeedSpec(t *testing.T) {
	tests := []struct {
		name    string
		opts    options
		want    feedSpec
		wantErr string // substring of the expected error, empty means no error
	}{
		{
			name: "defaults",
			opts: options{repo: "Homebrew/homebrew-cask"},
			want: feedSpec{Host: "github.com", Owner: "Homebrew", Repo: "homebrew-cask", State: "open"},
		},
		{
			name: "all filters",
			opts: options{
				repo:       "golang/go",
				host:       "ghe.corp.example.com",
				state:      "merged",
				labels:     stringList{"new cask", "ci-skip"},
				author:     "@matsuu",
				assignee:   "matsuu",
				milestone:  "Go1.27",
				qualifiers: stringList{"created:>2026-01-01"},
			},
			want: feedSpec{
				Host:       "ghe.corp.example.com",
				Owner:      "golang",
				Repo:       "go",
				State:      "merged",
				Labels:     []string{"new cask", "ci-skip"},
				Author:     "matsuu",
				Assignee:   "matsuu",
				Milestone:  "Go1.27",
				Qualifiers: []string{"created:>2026-01-01"},
			},
		},
		{
			name: "blank host and state fall back to defaults",
			opts: options{repo: "a/b", host: " ", state: " "},
			want: feedSpec{Host: defaultHost, Owner: "a", Repo: "b", State: defaultState},
		},
		{name: "missing repo", opts: options{}, wantErr: "-repo"},
		{name: "repo without name", opts: options{repo: "Homebrew"}, wantErr: "-repo"},
		{name: "repo with extra segment", opts: options{repo: "a/b/c"}, wantErr: "-repo"},
		{name: "repo is a URL", opts: options{repo: "https://github.com/Homebrew/homebrew-cask/pulls"}, wantErr: "-repo"},
		{name: "invalid state", opts: options{repo: "a/b", state: "bogus"}, wantErr: "-state"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newFeedSpec(tt.opts)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("newFeedSpec(%+v) error = %v, want error containing %q", tt.opts, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("newFeedSpec(%+v) failed: %v", tt.opts, err)
			}
			if got.Owner != tt.want.Owner || got.Repo != tt.want.Repo || got.Host != tt.want.Host ||
				got.State != tt.want.State || got.Author != tt.want.Author || got.Assignee != tt.want.Assignee ||
				got.Milestone != tt.want.Milestone {
				t.Errorf("newFeedSpec(%+v) = %+v, want %+v", tt.opts, got, tt.want)
			}
			if strings.Join(got.Labels, "|") != strings.Join(tt.want.Labels, "|") {
				t.Errorf("Labels = %v, want %v", got.Labels, tt.want.Labels)
			}
			if strings.Join(got.Qualifiers, "|") != strings.Join(tt.want.Qualifiers, "|") {
				t.Errorf("Qualifiers = %v, want %v", got.Qualifiers, tt.want.Qualifiers)
			}
		})
	}
}

func TestSearchQuery(t *testing.T) {
	tests := []struct {
		name string
		opts options
		want string
	}{
		{
			name: "default open",
			opts: options{repo: "Homebrew/homebrew-cask"},
			want: "repo:Homebrew/homebrew-cask is:pr is:open",
		},
		{
			name: "merged new cask",
			opts: options{repo: "Homebrew/homebrew-cask", state: "merged", labels: stringList{"new cask"}},
			want: `repo:Homebrew/homebrew-cask is:pr is:merged label:"new cask"`,
		},
		{
			name: "all",
			opts: options{repo: "golang/go", state: "all"},
			want: "repo:golang/go is:pr",
		},
		{
			name: "everything",
			opts: options{
				repo:       "golang/go",
				state:      "closed",
				labels:     stringList{"a", "b"},
				author:     "@matsuu",
				assignee:   "alice",
				milestone:  "Go1.27",
				qualifiers: stringList{"created:>2026-01-01", "-author:bob"},
			},
			want: `repo:golang/go is:pr is:closed label:"a" label:"b" author:matsuu assignee:alice milestone:"Go1.27" created:>2026-01-01 -author:bob`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, err := newFeedSpec(tt.opts)
			if err != nil {
				t.Fatalf("newFeedSpec(%+v) failed: %v", tt.opts, err)
			}
			if got := spec.searchQuery(); got != tt.want {
				t.Errorf("searchQuery() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPageQuery(t *testing.T) {
	tests := []struct {
		name string
		opts options
		want string
	}{
		{name: "default open is implied by the pulls page", opts: options{repo: "Homebrew/homebrew-cask"}, want: ""},
		{
			name: "merged with label",
			opts: options{repo: "Homebrew/homebrew-cask", state: "merged", labels: stringList{"new cask"}},
			want: `is:merged label:"new cask"`,
		},
		{name: "all", opts: options{repo: "a/b", state: "all"}, want: ""},
		{name: "author only", opts: options{repo: "a/b", author: "matsuu"}, want: "author:matsuu"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, err := newFeedSpec(tt.opts)
			if err != nil {
				t.Fatalf("newFeedSpec(%+v) failed: %v", tt.opts, err)
			}
			if got := spec.pageQuery(); got != tt.want {
				t.Errorf("pageQuery() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCanonicalURL(t *testing.T) {
	tests := []struct {
		name string
		opts options
		want string
	}{
		{
			name: "default",
			opts: options{repo: "Homebrew/homebrew-cask"},
			want: "https://github.com/Homebrew/homebrew-cask/pulls",
		},
		{
			name: "merged new cask",
			opts: options{repo: "Homebrew/homebrew-cask", state: "merged", labels: stringList{"new cask"}},
			want: "https://github.com/Homebrew/homebrew-cask/pulls?q=is%3Amerged+label%3A%22new+cask%22",
		},
		{
			name: "enterprise host",
			opts: options{repo: "team/repo", host: "ghe.corp.example.com"},
			want: "https://ghe.corp.example.com/team/repo/pulls",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, err := newFeedSpec(tt.opts)
			if err != nil {
				t.Fatalf("newFeedSpec(%+v) failed: %v", tt.opts, err)
			}
			if got := spec.canonicalURL(); got != tt.want {
				t.Errorf("canonicalURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAPIBase(t *testing.T) {
	tests := []struct {
		host string
		want string
	}{
		{"github.com", "https://api.github.com"},
		{"GitHub.com", "https://api.github.com"},
		{"ghe.corp.example.com", "https://ghe.corp.example.com/api/v3"},
	}
	for _, tt := range tests {
		spec := feedSpec{Host: tt.host}
		if got := spec.apiBase(); got != tt.want {
			t.Errorf("apiBase() for %q = %q, want %q", tt.host, got, tt.want)
		}
	}
}
