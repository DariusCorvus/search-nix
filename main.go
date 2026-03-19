package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func main() {
	channel := ""
	size := 20
	verbose := false
	tui := false
	var positional []string

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-c", "--channel":
			if i+1 >= len(args) {
				fatal("missing value for %s", args[i])
			}
			i++
			channel = args[i]
		case "-n", "--size":
			if i+1 >= len(args) {
				fatal("missing value for %s", args[i])
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil {
				fatal("invalid size: %s", args[i])
			}
			size = n
		case "-v", "--verbose":
			verbose = true
		case "-t", "--tui":
			tui = true
		case "-h", "--help":
			printUsage()
			os.Exit(0)
		default:
			positional = append(positional, args[i])
		}
	}

	query := strings.Join(positional, " ")

	if channel == "" {
		channel = detectChannel()
	}

	if tui {
		altChannel := detectAltChannel(channel)
		os.Exit(runTUI(channel, altChannel, size, query))
	}

	if query == "" {
		fmt.Fprintln(os.Stderr, "error: no search query provided")
		fmt.Fprintln(os.Stderr, "Usage: search-nix <query>")
		os.Exit(1)
	}

	resp, elapsed, err := search(query, channel, size)
	if err != nil {
		fatal("%v", err)
	}

	total := resp.Hits.Total.Value
	results := resp.Hits.Hits

	if total == 0 {
		fmt.Printf("no results for '%s' on channel %s\n", query, channel)
		return
	}

	opts := RenderOpts{
		Query:   query,
		Channel: channel,
		Total:   total,
		Elapsed: elapsed,
	}

	if verbose {
		renderResultsVerbose(results, opts)
	} else {
		renderResults(results, opts)
	}
}

func printUsage() {
	fmt.Print(`Usage: search-nix [OPTIONS] [query]

Options:
  -c, --channel <channel>   Channel to search (unstable, 25.11, 24.11, ...)
                            Default: auto-detected from nixos-version
  -n, --size <n>            Number of results (default: 20, max: 50)
  -v, --verbose             Show full output (homepage, license, programs)
  -t, --tui                 Launch interactive TUI mode
  -h, --help                Show this help

Examples:
  search-nix fuser
  search-nix -v fuser
  search-nix -n 5 python linter
  search-nix -c unstable ffmpeg
  search-nix -t                    # interactive TUI
  search-nix -t fuser              # TUI with pre-filled query
`)
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
