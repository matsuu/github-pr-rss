package main

import (
	"strings"
	"testing"
	"time"
)

func sampleResult() ghSearchResult {
	merged := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	created := time.Date(2026, 9, 30, 9, 0, 0, 0, time.UTC)
	return ghSearchResult{
		TotalCount: 1,
		Items: []ghIssue{
			{
				Number:      42,
				Title:       "Add foo 1.2.3",
				HTMLURL:     "https://github.com/Homebrew/homebrew-cask/pull/42",
				Body:        "Adds <foo> & more",
				User:        ghUser{Login: "alice"},
				Labels:      []ghLabel{{Name: "new cask"}},
				CreatedAt:   created,
				PullRequest: &ghPullRequest{MergedAt: &merged},
			},
		},
	}
}

func sampleSpec(t *testing.T) feedSpec {
	t.Helper()
	spec, err := newFeedSpec(options{
		repo:   "Homebrew/homebrew-cask",
		state:  "merged",
		labels: stringList{"new cask"},
	})
	if err != nil {
		t.Fatalf("newFeedSpec: %v", err)
	}
	return spec
}

func TestBuildFeedAndMarshal(t *testing.T) {
	spec := sampleSpec(t)
	now := time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)

	feed := buildFeed(spec, sampleResult(), now, 0)
	data, err := marshalFeed(feed, false)
	if err != nil {
		t.Fatalf("marshalFeed: %v", err)
	}
	s := string(data)

	// encoding/xml escapes `"` in chardata as &#34;
	checks := []string{
		`<?xml version="1.0" encoding="UTF-8"?>`,
		`<rss version="2.0" xmlns:dc="http://purl.org/dc/elements/1.1/">`,
		`<title>Homebrew/homebrew-cask pull requests: is:merged label:&#34;new cask&#34;</title>`,
		`<link>https://github.com/Homebrew/homebrew-cask/pulls?q=is%3Amerged+label%3A%22new+cask%22</link>`,
		`<description>Pull requests matching repo:Homebrew/homebrew-cask is:pr is:merged label:&#34;new cask&#34; on GitHub</description>`,
		`<lastBuildDate>` + now.Format(time.RFC1123Z) + `</lastBuildDate>`,
		`<generator>github-pr-rss ` + version + `</generator>`,
		`<item>`,
		`<title>PR #42: Add foo 1.2.3</title>`,
		`<link>https://github.com/Homebrew/homebrew-cask/pull/42</link>`,
		`<description>Adds &lt;foo&gt; &amp; more</description>`,
		`<dc:creator>alice</dc:creator>`,
		`<category>new cask</category>`,
		`<guid>https://github.com/Homebrew/homebrew-cask/pull/42</guid>`,
		// merged date is used when the PR was merged
		`<pubDate>Thu, 01 Oct 2026 12:00:00 +0000</pubDate>`,
		`</item>`,
	}
	for _, want := range checks {
		if !strings.Contains(s, want) {
			t.Errorf("output XML does not contain %q\nXML:\n%s", want, s)
		}
	}
}

func TestChannelPubDateIsNewestItem(t *testing.T) {
	spec, err := newFeedSpec(options{repo: "a/b"})
	if err != nil {
		t.Fatalf("newFeedSpec: %v", err)
	}
	older := ghIssue{Number: 1, HTMLURL: "https://github.com/a/b/pull/1",
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	newest := ghIssue{Number: 2, HTMLURL: "https://github.com/a/b/pull/2",
		CreatedAt: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)}
	// ascending order: the newest item comes last
	feed := buildFeed(spec, ghSearchResult{Items: []ghIssue{older, newest}}, time.Now(), 0)
	if want := newest.CreatedAt.Format(time.RFC1123Z); feed.Channel.PubDate != want {
		t.Errorf("channel PubDate = %q, want newest item date %q", feed.Channel.PubDate, want)
	}
}

func TestMarshalFeedPretty(t *testing.T) {
	spec, err := newFeedSpec(options{repo: "Homebrew/homebrew-cask"})
	if err != nil {
		t.Fatalf("newFeedSpec: %v", err)
	}
	feed := buildFeed(spec, sampleResult(), time.Now(), 0)
	data, err := marshalFeed(feed, true)
	if err != nil {
		t.Fatalf("marshalFeed: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, "\n  <channel>") {
		t.Errorf("pretty output missing indented <channel>:\n%s", s)
	}
}

func TestEmptyFeed(t *testing.T) {
	spec, err := newFeedSpec(options{repo: "Homebrew/homebrew-cask"})
	if err != nil {
		t.Fatalf("newFeedSpec: %v", err)
	}
	feed := buildFeed(spec, ghSearchResult{}, time.Now(), 0)
	data, err := marshalFeed(feed, false)
	if err != nil {
		t.Fatalf("marshalFeed: %v", err)
	}
	s := string(data)
	if strings.Contains(s, "<item>") {
		t.Errorf("empty result produced items:\n%s", s)
	}
	if strings.Contains(s, "<pubDate>") {
		t.Errorf("empty result should not have channel pubDate:\n%s", s)
	}
}

func TestTruncate(t *testing.T) {
	long := strings.Repeat("あ", 20) // 20 runes, 60 bytes
	got := truncate(long, 10)
	if want := strings.Repeat("あ", 10) + "\n\n[description truncated by github-pr-rss]"; got != want {
		t.Errorf("truncate() = %q, want %q", got, want)
	}
	if got := truncate("short", 10); got != "short" {
		t.Errorf("truncate(short) = %q, want unchanged", got)
	}
	if got := truncate("short", 0); got != "short" {
		t.Errorf("truncate with n=0 = %q, want unchanged", got)
	}
}

func TestNewItemDates(t *testing.T) {
	merged := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	// merged PR uses the merge date
	is := ghIssue{CreatedAt: merged.AddDate(0, 0, -2), PullRequest: &ghPullRequest{MergedAt: &merged}}
	item := newRSSItem(is, 0)
	if want := "Thu, 01 Oct 2026 12:00:00 +0000"; item.PubDate != want {
		t.Errorf("merged PR PubDate = %q, want %q", item.PubDate, want)
	}
	// open PR falls back to the creation date
	is = ghIssue{CreatedAt: merged}
	item = newRSSItem(is, 0)
	if item.PubDate != "Thu, 01 Oct 2026 12:00:00 +0000" {
		t.Errorf("open PR PubDate = %q, want creation date", item.PubDate)
	}
}
