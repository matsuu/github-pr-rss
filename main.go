// Command github-pr-rss generates an RSS 2.0 feed of GitHub pull requests.
//
// For example, a feed of merged homebrew-cask pull requests that added a
// new cask:
//
//	github-pr-rss -repo Homebrew/homebrew-cask -state merged -label "new cask"
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"
)

const (
	progName = "github-pr-rss"
	version  = "1.0.0"

	defaultHost  = "github.com"
	defaultState = "open"
)

// errUsagePrinted signals that usage or version info was already printed
// and the program should exit successfully.
var errUsagePrinted = errors.New("usage already printed")

// errFlagError signals that the flag package already reported a parsing
// error (message and usage) to the error output; the program should exit
// with a usage-failure status without printing the error again.
var errFlagError = errors.New("flag error already reported")

// options holds all command-line flags.
type options struct {
	repo       string
	host       string
	state      string
	labels     stringList
	author     string
	assignee   string
	milestone  string
	qualifiers stringList

	limit   int
	maxDesc int
	sort    string
	order   string
	token   string
	api     string
	out     string
	pretty  bool
}

// stringList is a flag value that can be repeated to collect multiple
// strings.
type stringList []string

func (l *stringList) String() string { return strings.Join(*l, ", ") }

func (l *stringList) Set(v string) error {
	*l = append(*l, v)
	return nil
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr, os.Getenv); err != nil {
		if errors.Is(err, errFlagError) {
			os.Exit(2)
		}
		fmt.Fprintln(os.Stderr, progName+":", err)
		os.Exit(1)
	}
}

func run(args []string, out, errOut io.Writer, getenv func(string) string) error {
	opts, err := parseFlags(args, out, errOut)
	if err != nil {
		if errors.Is(err, errUsagePrinted) {
			return nil
		}
		return err
	}

	spec, err := newFeedSpec(opts)
	if err != nil {
		return err
	}

	token := firstNonEmpty(opts.token, getenv("GITHUB_TOKEN"), getenv("GH_TOKEN"))
	apiBase := opts.api
	if apiBase == "" {
		apiBase = spec.apiBase()
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	client := &ghClient{
		HTTP:    &http.Client{Timeout: 30 * time.Second},
		APIBase: apiBase,
		Token:   token,
	}

	res, err := client.searchIssues(ctx, spec.searchQuery(), opts.sort, opts.order, opts.limit)
	if err != nil {
		return err
	}

	if res.IncompleteResults {
		fmt.Fprintln(errOut, "warning: GitHub reports these search results may be incomplete; try again later")
	}
	if res.TotalCount > len(res.Items) {
		fmt.Fprintf(errOut, "note: %d pull requests match this query; the feed contains the first %d (GitHub's search API returns at most 1000)\n",
			res.TotalCount, len(res.Items))
	}
	if len(res.Items) == 0 {
		fmt.Fprintln(errOut, "note: no pull requests matched the query")
	}

	feed := buildFeed(spec, res, time.Now(), opts.maxDesc)
	data, err := marshalFeed(feed, opts.pretty)
	if err != nil {
		return err
	}
	if opts.out == "" || opts.out == "-" {
		_, err = out.Write(data)
		return err
	}
	return writeFileAtomically(opts.out, data)
}

func parseFlags(args []string, out, errOut io.Writer) (options, error) {
	fs := flag.NewFlagSet(progName, flag.ContinueOnError)
	fs.SetOutput(errOut)
	var opts options
	fs.StringVar(&opts.repo, "repo", "", "GitHub repository in `<owner>/<name>` format (required)")
	fs.StringVar(&opts.host, "host", defaultHost, "GitHub `host` ("+defaultHost+" or a GitHub Enterprise host)")
	fs.StringVar(&opts.state, "state", defaultState, "pull request `state`: open, closed, merged, unmerged, or all")
	fs.Var(&opts.labels, "label", "filter by `label`; repeat the flag to combine labels")
	fs.StringVar(&opts.author, "author", "", "filter by pull request `author` (login)")
	fs.StringVar(&opts.assignee, "assignee", "", "filter by `assignee` (login)")
	fs.StringVar(&opts.milestone, "milestone", "", "filter by `milestone` (name)")
	fs.Var(&opts.qualifiers, "qualifier", "extra GitHub search `qualifier`; repeatable (e.g. 'created:>2026-01-01')")
	fs.IntVar(&opts.limit, "limit", 50, "maximum `number` of pull requests in the feed (0 = everything the API returns, up to 1000)")
	fs.IntVar(&opts.maxDesc, "max-desc", 4000, "truncate each PR body to about this many `characters` (0 = no truncation)")
	fs.StringVar(&opts.sort, "sort", "created", "`sort` field for results: created, updated, or comments")
	fs.StringVar(&opts.order, "order", "desc", "sort `direction`: desc or asc")
	fs.StringVar(&opts.token, "token", "", "GitHub access `token` (defaults to $GITHUB_TOKEN / $GH_TOKEN)")
	fs.StringVar(&opts.api, "api", "", "base `URL` of the GitHub REST API (default: derived from -host)")
	fs.StringVar(&opts.out, "out", "", "write the feed to this `file` instead of stdout (\"-\" for stdout); an existing file is replaced atomically")
	fs.BoolVar(&opts.pretty, "pretty", false, "indent the output XML")
	showVersion := fs.Bool("version", false, "print version and exit")
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), usage)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return opts, errUsagePrinted // -h: usage already printed
		}
		// The flag package already printed the error and the usage text.
		return opts, errFlagError
	}
	if *showVersion {
		fmt.Fprintln(out, progName+" "+version)
		return opts, errUsagePrinted
	}
	if fs.NArg() > 0 {
		return opts, fmt.Errorf("unexpected argument %q: pass the repository with -repo and filters as flags", fs.Arg(0))
	}
	switch opts.sort {
	case "created", "updated", "comments":
	default:
		return opts, fmt.Errorf("invalid -sort %q: want created, updated, or comments", opts.sort)
	}
	switch opts.order {
	case "desc", "asc":
	default:
		return opts, fmt.Errorf("invalid -order %q: want desc or asc", opts.order)
	}
	if opts.limit < 0 {
		return opts, fmt.Errorf("invalid -limit %d: want 0 or more", opts.limit)
	}
	if opts.maxDesc < 0 {
		return opts, fmt.Errorf("invalid -max-desc %d: want 0 or more", opts.maxDesc)
	}
	return opts, nil
}

const usage = progName + ` generates an RSS 2.0 feed of GitHub pull requests.

Usage:

  ` + progName + ` -repo <owner>/<name> [flags]

Examples:

  # Homebrew: recently merged PRs that added a new cask
  ` + progName + ` -repo Homebrew/homebrew-cask -state merged -label "new cask"

  # Homebrew: recently merged PRs that added a new formula, written to a file
  ` + progName + ` -repo Homebrew/homebrew-core -state merged -label "new formula" -out homebrew-core.rss

  # golang/go: open PRs for a milestone by one author
  ` + progName + ` -repo golang/go -milestone Go1.27 -author matsuu

The feed is written to stdout, or to the file given by -out (an existing
file is replaced atomically). Without a token, the search API allows only
10 requests per minute — set GITHUB_TOKEN (or GH_TOKEN) in the environment
for a higher rate limit.

Flags:
`

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
