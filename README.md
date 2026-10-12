# github-pr-rss

A command-line tool that generates an RSS 2.0 feed from a GitHub pull request list. Specify the repository and filter conditions via flags. It has no external dependencies (Go standard library only).

## Usage

```console
$ go build .
$ ./github-pr-rss -repo Homebrew/homebrew-cask -state merged -label 'new cask' > homebrew-cask.rss
$ ./github-pr-rss -repo Homebrew/homebrew-core -state merged -label 'new formula' -out homebrew-core.rss
```

By default the generated feed is written to stdout; redirect it to a file, or use `-out` to write it directly.

When `-out` targets an existing file, the feed is written to a temporary file in the same directory and then swapped in atomically, so concurrent readers never see a partially written file. Permissions of an existing file are preserved; new files are created with mode 0644.

### Flags

| Flag | Description |
|---|---|
| `-repo <owner>/<name>` | Target repository (required). Example: `Homebrew/homebrew-cask` |
| `-state <state>` | Pull request state: `open` (default), `closed`, `merged`, `unmerged`, or `all` (no restriction) |
| `-label <name>` | Filter by label. May be repeated; multiple labels are combined with AND |
| `-author <login>` | Filter by pull request author (a leading `@` is accepted) |
| `-assignee <login>` | Filter by assignee |
| `-milestone <name>` | Filter by milestone |
| `-qualifier <q>` | Extra GitHub search qualifier; may be repeated (e.g. `-qualifier 'created:>2026-01-01'`) |
| `-host <host>` | GitHub host (default: `github.com`). Works with GitHub Enterprise |
| `-limit <n>` | Maximum number of pull requests in the feed (default: 50; 0 = as many as available, up to the API limit of 1000) |
| `-max-desc <n>` | Truncate each PR body to this many characters (default: 4000; 0 = unlimited) |
| `-sort <field>` | Sort field: `created`, `updated`, or `comments` (default: `created`) |
| `-order <dir>` | Sort direction: `desc` or `asc` (default: `desc`) |
| `-token <token>` | GitHub access token (default: `$GITHUB_TOKEN` / `$GH_TOKEN`) |
| `-api <url>` | Base URL of the GitHub REST API (default: derived from `-host`) |
| `-out <file>` | Write the feed to this file instead of stdout (`-` for stdout). An existing file is replaced atomically |
| `-pretty` | Indent the output XML |
| `-version` | Print the version |

### Examples

```console
# Open pull requests for the Go1.27 milestone on golang/go
$ ./github-pr-rss -repo golang/go -milestone Go1.27

# Open pull requests created since 2026
$ ./github-pr-rss -repo Homebrew/homebrew-cask -qualifier 'created:>2026-01-01'

# Closed pull requests by a specific author (top 100)
$ ./github-pr-rss -repo golang/go -state closed -author matsuu -limit 100
```

### Tokens and rate limits

Without a token, the GitHub search API allows only **10 requests per minute**. Set the `GITHUB_TOKEN` (or `GH_TOKEN`) environment variable, or pass `-token`, to raise the limit to 30 requests per minute.

```console
$ export GITHUB_TOKEN=ghp_xxxxxxxx
$ ./github-pr-rss -repo Homebrew/homebrew-cask -state merged -label 'new cask' > feed.rss
```

### Periodic execution

Example cron entry that regenerates the feed hourly:

```cron
0 * * * * cd /path/to/github-pr-rss && ./github-pr-rss -repo Homebrew/homebrew-cask -state merged -label 'new cask' -limit 100 -out /var/www/rss/homebrew-cask.rss 2>> /var/log/github-pr-rss.log
```

## How it works

1. Builds a GitHub search API query from the flags (e.g. `repo:Homebrew/homebrew-cask is:pr is:merged label:"new cask"`)
2. Fetches the pull request list from `https://api.github.com/search/issues?q=…` (with pagination)
3. Converts each pull request into an RSS item and writes an RSS 2.0 document

Each item is structured as follows:

| Element | Content |
|---|---|
| `title` | `PR #<number>: <title>` |
| `link` / `guid` | URL of the pull request |
| `description` | PR body (truncated by `-max-desc`) |
| `dc:creator` | PR author |
| `category` | Labels |
| `pubDate` | Merge time for merged PRs; creation time otherwise |

The channel `link` points to the GitHub pulls page URL with the equivalent filter conditions.

GitHub Enterprise is supported: pass `-host ghe.example.com` and the tool uses `https://ghe.example.com/api/v3` as the API endpoint.

## Limitations

- The GitHub search API returns at most 1000 results per query (even with `-limit 0`)
- `description` contains the raw Markdown source of the PR body
- Labels and milestones containing double quotes cannot be searched (GitHub search syntax has no escaping) and are rejected

## Development

```console
$ go test ./...
$ go vet ./...
```
