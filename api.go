package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const ghAPIVersion = "2022-11-28"

// ghUser is the pull request author.
type ghUser struct {
	Login string `json:"login"`
}

// ghLabel is a GitHub label.
type ghLabel struct {
	Name string `json:"name"`
}

// ghPullRequest carries the fields that only pull requests have.
type ghPullRequest struct {
	MergedAt *time.Time `json:"merged_at"`
}

// ghIssue is a pull request as returned by the GitHub search API.
type ghIssue struct {
	Number      int            `json:"number"`
	Title       string         `json:"title"`
	HTMLURL     string         `json:"html_url"`
	Body        string         `json:"body"`
	User        ghUser         `json:"user"`
	Labels      []ghLabel      `json:"labels"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	ClosedAt    *time.Time     `json:"closed_at"`
	PullRequest *ghPullRequest `json:"pull_request"`
}

// ghSearchResult is the payload of GET /search/issues.
type ghSearchResult struct {
	TotalCount        int       `json:"total_count"`
	IncompleteResults bool      `json:"incomplete_results"`
	Items             []ghIssue `json:"items"`
}

// ghClient is a minimal GitHub REST API client.
type ghClient struct {
	HTTP    *http.Client
	APIBase string // e.g. https://api.github.com
	Token   string // optional access token
}

// searchIssues lists up to max issues/PRs matching the search query q,
// following the search API's pagination. A max of 0 or less means "as many
// as available"; the search API never returns more than 1000 results.
func (c *ghClient) searchIssues(ctx context.Context, q, sort, order string, max int) (ghSearchResult, error) {
	var res ghSearchResult
	for page := 1; ; page++ {
		perPage := 100
		if max > 0 {
			need := max - len(res.Items)
			if need < 1 {
				break
			}
			if perPage > need {
				perPage = need
			}
		}

		u := strings.TrimSuffix(c.APIBase, "/") + "/search/issues?q=" + url.QueryEscape(q)
		if sort != "" {
			u += "&sort=" + url.QueryEscape(sort)
		}
		if order != "" {
			u += "&order=" + url.QueryEscape(order)
		}
		u += "&per_page=" + strconv.Itoa(perPage) + "&page=" + strconv.Itoa(page)

		var r ghSearchResult
		if err := c.getJSON(ctx, u, &r); err != nil {
			return res, err
		}
		if page == 1 {
			res.TotalCount = r.TotalCount
			res.IncompleteResults = r.IncompleteResults
		}
		if len(r.Items) == 0 {
			break
		}
		res.Items = append(res.Items, r.Items...)
		if max > 0 && len(res.Items) >= max {
			break
		}
		if len(r.Items) < perPage {
			break
		}
		if len(res.Items) >= 1000 { // GitHub search API hard cap
			break
		}
	}
	if max > 0 && len(res.Items) > max {
		res.Items = res.Items[:max]
	}
	return res, nil
}

// getJSON performs a GET request and decodes the JSON response into v.
func (c *ghClient) getJSON(ctx context.Context, u string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", ghAPIVersion)
	req.Header.Set("User-Agent", progName+"/"+version)
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return fmt.Errorf("reading response from %s: %w", u, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return apiError(resp, body, c.Token != "")
	}
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("decoding response from %s: %w", u, err)
	}
	return nil
}

// apiError renders a GitHub API error response as a helpful error value.
func apiError(resp *http.Response, body []byte, authed bool) error {
	var e struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal(body, &e)
	msg := strings.TrimSpace(e.Message)
	if msg == "" {
		msg = firstNonEmpty(cutRunes(strings.TrimSpace(string(body)), 300), resp.Status)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "GitHub API request failed: %s: %s", resp.Status, msg)
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		if reset := resp.Header.Get("X-RateLimit-Reset"); reset != "" {
			if sec, err := strconv.ParseInt(reset, 10, 64); err == nil {
				fmt.Fprintf(&b, " (rate limit resets at %s)",
					time.Unix(sec, 0).Local().Format("2006-01-02 15:04:05 MST"))
			}
		}
		if !authed {
			b.WriteString("; without a token the search API allows only 10 requests per minute — set GITHUB_TOKEN or pass -token")
		}
	}
	return errors.New(b.String())
}

// cutRunes returns s truncated to at most n runes.
func cutRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
