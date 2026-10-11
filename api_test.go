package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSearchIssuesFollowsPagination(t *testing.T) {
	const total = 101
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search/issues" {
			t.Errorf("request path = %q, want /search/issues", r.URL.Path)
		}
		if got, want := r.URL.Query().Get("q"), "repo:Homebrew/homebrew-cask is:pr"; got != want {
			t.Errorf("q = %q, want %q", got, want)
		}
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))

		res := ghSearchResult{TotalCount: total}
		for i := 0; i < perPage && (page-1)*perPage+i < total; i++ {
			n := (page-1)*perPage + i + 1
			res.Items = append(res.Items, ghIssue{
				Number:    n,
				Title:     fmt.Sprintf("PR %d", n),
				HTMLURL:   fmt.Sprintf("https://github.com/Homebrew/homebrew-cask/pull/%d", n),
				CreatedAt: time.Date(2026, 10, 1, 0, 0, n, 0, time.UTC),
			})
		}
		json.NewEncoder(w).Encode(res)
	}))
	defer srv.Close()

	c := &ghClient{HTTP: srv.Client(), APIBase: srv.URL}
	res, err := c.searchIssues(context.Background(), "repo:Homebrew/homebrew-cask is:pr", "created", "desc", 0)
	if err != nil {
		t.Fatalf("searchIssues: %v", err)
	}
	if res.TotalCount != total {
		t.Errorf("TotalCount = %d, want %d", res.TotalCount, total)
	}
	if len(res.Items) != total {
		t.Errorf("len(Items) = %d, want %d", len(res.Items), total)
	}
	if res.Items[0].Number != 1 || res.Items[total-1].Number != total {
		t.Errorf("items out of order: first=%d last=%d", res.Items[0].Number, res.Items[total-1].Number)
	}
}

func TestSearchIssuesLimit(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
		if perPage != 5 {
			t.Errorf("per_page = %d, want 5", perPage)
		}
		res := ghSearchResult{TotalCount: 250}
		for i := 1; i <= perPage; i++ {
			res.Items = append(res.Items, ghIssue{Number: i, HTMLURL: fmt.Sprintf("https://github.com/o/r/pull/%d", i)})
		}
		json.NewEncoder(w).Encode(res)
	}))
	defer srv.Close()

	c := &ghClient{HTTP: srv.Client(), APIBase: srv.URL}
	res, err := c.searchIssues(context.Background(), "repo:o/r is:pr", "", "", 5)
	if err != nil {
		t.Fatalf("searchIssues: %v", err)
	}
	if len(res.Items) != 5 {
		t.Errorf("len(Items) = %d, want 5", len(res.Items))
	}
	if got := requests.Load(); got != 1 {
		t.Errorf("made %d requests, want 1", got)
	}
}

func TestClientHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q, want %q", got, "Bearer test-token")
		}
		if got := r.Header.Get("Accept"); got != "application/vnd.github+json" {
			t.Errorf("Accept = %q", got)
		}
		if got := r.Header.Get("X-GitHub-Api-Version"); got != ghAPIVersion {
			t.Errorf("X-GitHub-Api-Version = %q, want %q", got, ghAPIVersion)
		}
		if got := r.Header.Get("User-Agent"); got == "" {
			t.Error("User-Agent is empty")
		}
		json.NewEncoder(w).Encode(ghSearchResult{})
	}))
	defer srv.Close()

	c := &ghClient{HTTP: srv.Client(), APIBase: srv.URL, Token: "test-token"}
	if _, err := c.searchIssues(context.Background(), "repo:o/r is:pr", "", "", 5); err != nil {
		t.Fatalf("searchIssues: %v", err)
	}
}

func TestRateLimitError(t *testing.T) {
	reset := time.Now().Add(time.Hour).Unix()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(reset, 10))
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"message": "API rate limit exceeded for 192.0.2.1"})
	}))
	defer srv.Close()

	c := &ghClient{HTTP: srv.Client(), APIBase: srv.URL}
	_, err := c.searchIssues(context.Background(), "repo:o/r is:pr", "", "", 5)
	if err == nil {
		t.Fatal("searchIssues succeeded, want rate-limit error")
	}
	for _, want := range []string{"rate limit", "GITHUB_TOKEN", "10 requests per minute"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}
