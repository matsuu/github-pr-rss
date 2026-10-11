package main

import (
	"fmt"
	"net/url"
	"strings"
)

// feedSpec describes the set of GitHub pull requests to include in a feed.
type feedSpec struct {
	Host       string
	Owner      string
	Repo       string
	State      string // open, closed, merged, unmerged, all
	Labels     []string
	Author     string
	Assignee   string
	Milestone  string
	Qualifiers []string // extra raw search qualifiers
}

// stateQualifiers maps -state values to GitHub search qualifiers. "all"
// imposes no state restriction.
var stateQualifiers = map[string]string{
	"open":     "is:open",
	"closed":   "is:closed",
	"merged":   "is:merged",
	"unmerged": "is:unmerged",
	"all":      "",
}

// newFeedSpec validates the command-line options and turns them into a
// feedSpec.
func newFeedSpec(o options) (feedSpec, error) {
	owner, name, err := splitRepoName(o.repo)
	if err != nil {
		return feedSpec{}, err
	}
	state := strings.TrimSpace(o.state)
	if state == "" {
		state = defaultState
	}
	if _, ok := stateQualifiers[state]; !ok {
		return feedSpec{}, fmt.Errorf("invalid -state %q: want open, closed, merged, unmerged, or all", o.state)
	}
	host := strings.TrimSpace(o.host)
	if host == "" {
		host = defaultHost
	}
	return feedSpec{
		Host:       host,
		Owner:      owner,
		Repo:       name,
		State:      state,
		Labels:     nonEmptyTrimmed(o.labels),
		Author:     cleanLogin(o.author),
		Assignee:   cleanLogin(o.assignee),
		Milestone:  strings.TrimSpace(o.milestone),
		Qualifiers: nonEmptyTrimmed(o.qualifiers),
	}, nil
}

// splitRepoName splits a "<owner>/<name>" repository reference.
func splitRepoName(repo string) (string, string, error) {
	parts := strings.Split(strings.TrimSpace(repo), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid -repo %q: want <owner>/<name>, e.g. Homebrew/homebrew-cask", repo)
	}
	return parts[0], parts[1], nil
}

// searchQuery returns the query for the GitHub search API.
func (s feedSpec) searchQuery() string {
	qs := []string{"repo:" + s.Owner + "/" + s.Repo, "is:pr"}
	if q := stateQualifiers[s.State]; q != "" {
		qs = append(qs, q)
	}
	qs = append(qs, s.userQualifiers()...)
	return strings.Join(qs, " ")
}

// pageQuery returns the ?q= value that reproduces the same list on the
// GitHub pulls page, which already implies the repository, pull requests,
// and (by default) the open state.
func (s feedSpec) pageQuery() string {
	var qs []string
	if q := stateQualifiers[s.State]; q != "" && s.State != "open" {
		qs = append(qs, q)
	}
	qs = append(qs, s.userQualifiers()...)
	return strings.Join(qs, " ")
}

// userQualifiers returns the label/author/milestone/… qualifiers.
func (s feedSpec) userQualifiers() []string {
	var qs []string
	for _, l := range s.Labels {
		qs = append(qs, `label:"`+l+`"`)
	}
	if s.Author != "" {
		qs = append(qs, "author:"+s.Author)
	}
	if s.Assignee != "" {
		qs = append(qs, "assignee:"+s.Assignee)
	}
	if s.Milestone != "" {
		qs = append(qs, `milestone:"`+s.Milestone+`"`)
	}
	qs = append(qs, s.Qualifiers...)
	return qs
}

// canonicalURL returns the address of the equivalent GitHub pulls page.
func (s feedSpec) canonicalURL() string {
	u := url.URL{
		Scheme: "https",
		Host:   s.Host,
		Path:   "/" + s.Owner + "/" + s.Repo + "/pulls",
	}
	if q := s.pageQuery(); q != "" {
		u.RawQuery = "q=" + url.QueryEscape(q)
	}
	return u.String()
}

// apiBase returns the REST API base URL for the GitHub instance:
// api.github.com for github.com, /api/v3 for GitHub Enterprise.
func (s feedSpec) apiBase() string {
	host := strings.ToLower(s.Host)
	if host == "github.com" || host == "www.github.com" {
		return "https://api.github.com"
	}
	return "https://" + s.Host + "/api/v3"
}

// cleanLogin trims a login, dropping an optional leading "@".
func cleanLogin(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "@")
}

// nonEmptyTrimmed returns the values trimmed, dropping empty ones.
func nonEmptyTrimmed(vals []string) []string {
	var out []string
	for _, v := range vals {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}
