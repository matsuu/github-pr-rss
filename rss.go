package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"time"
	"unicode/utf8"
)

const dcNamespace = "http://purl.org/dc/elements/1.1/"

type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	DC      string     `xml:"xmlns:dc,attr"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	XMLName       xml.Name  `xml:"channel"`
	Title         string    `xml:"title"`
	Link          string    `xml:"link"`
	Description   string    `xml:"description"`
	PubDate       string    `xml:"pubDate,omitempty"`
	LastBuildDate string    `xml:"lastBuildDate,omitempty"`
	Generator     string    `xml:"generator,omitempty"`
	Items         []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string   `xml:"title"`
	Link        string   `xml:"link"`
	Description string   `xml:"description,omitempty"`
	Creator     string   `xml:"dc:creator,omitempty"`
	Categories  []string `xml:"category,omitempty"`
	GUID        string   `xml:"guid"`
	PubDate     string   `xml:"pubDate"`
}

// buildFeed assembles the RSS feed for the given pull requests. Each
// item is dated by its merge time when the PR was merged, and by its
// creation time otherwise. The channel pubDate is the newest item date,
// which is not necessarily the first item when sorting ascending.
func buildFeed(spec feedSpec, res ghSearchResult, now time.Time, maxDesc int) *rssFeed {
	title := spec.Owner + "/" + spec.Repo + " pull requests"
	if q := spec.pageQuery(); q != "" {
		title += ": " + q
	}

	ch := rssChannel{
		Title:         title,
		Link:          spec.canonicalURL(),
		Description:   "Pull requests matching " + spec.searchQuery() + " on GitHub",
		LastBuildDate: now.UTC().Format(time.RFC1123Z),
		Generator:     progName + " " + version,
	}
	var latest time.Time
	for _, is := range res.Items {
		if d := itemDate(is); d.After(latest) {
			latest = d
		}
		ch.Items = append(ch.Items, newRSSItem(is, maxDesc))
	}
	if !latest.IsZero() {
		ch.PubDate = latest.UTC().Format(time.RFC1123Z)
	}
	return &rssFeed{Version: "2.0", DC: dcNamespace, Channel: ch}
}

func newRSSItem(is ghIssue, maxDesc int) rssItem {
	var categories []string
	for _, l := range is.Labels {
		categories = append(categories, l.Name)
	}
	return rssItem{
		Title:       fmt.Sprintf("PR #%d: %s", is.Number, is.Title),
		Link:        is.HTMLURL,
		Description: truncate(is.Body, maxDesc),
		Creator:     is.User.Login,
		Categories:  categories,
		GUID:        is.HTMLURL,
		PubDate:     itemDate(is).UTC().Format(time.RFC1123Z),
	}
}

// itemDate returns the date an item is published under: the merge time
// when the pull request was merged, and the creation time otherwise.
func itemDate(is ghIssue) time.Time {
	if is.PullRequest != nil && is.PullRequest.MergedAt != nil {
		return *is.PullRequest.MergedAt
	}
	return is.CreatedAt
}

// truncate shortens s to at most n runes, marking the cut.
func truncate(s string, n int) string {
	if n <= 0 || utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "\n\n[description truncated by " + progName + "]"
}

// marshalFeed serializes the feed to XML, optionally indented.
func marshalFeed(f *rssFeed, pretty bool) ([]byte, error) {
	var buf bytes.Buffer
	enc := xml.NewEncoder(&buf)
	if pretty {
		enc.Indent("", "  ")
	}
	if err := enc.Encode(f); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	out := make([]byte, 0, len(xml.Header)+buf.Len()+1)
	out = append(out, xml.Header...)
	out = append(out, buf.Bytes()...)
	if out[len(out)-1] != '\n' {
		out = append(out, '\n')
	}
	return out, nil
}
