package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"golang.org/x/term"
)

// ANSI escape codes
const (
	bold    = "\033[1m"
	dim     = "\033[2m"
	ul      = "\033[4m"
	reset   = "\033[0m"
	cyan    = "\033[36m"
	green   = "\033[32m"
	yellow  = "\033[33m"
	magenta = "\033[35m"
)

type RenderOpts struct {
	Query   string
	Channel string
	Total   int
	Elapsed time.Duration
}

var hasColor bool

func init() {
	hasColor = term.IsTerminal(int(os.Stdout.Fd()))
}

func c(code, s string) string {
	if !hasColor {
		return s
	}
	return code + s + reset
}

func highlight(s string, query string) string {
	if !hasColor || query == "" {
		return s
	}
	escaped := regexp.QuoteMeta(query)
	re, err := regexp.Compile("(?i)" + escaped)
	if err != nil {
		return s
	}
	return re.ReplaceAllStringFunc(s, func(match string) string {
		return bold + ul + match + reset
	})
}

func renderResults(hits []ESHit, opts RenderOpts) {
	count := len(hits)

	// Reverse: best match last (bottom of terminal), numbered [1] = best
	for i := count - 1; i >= 0; i-- {
		p := hits[i].Source
		num := i + 1

		fmt.Println(c(dim, "───"))
		fmt.Printf("%s  %s  %s\n",
			c(cyan, fmt.Sprintf("[%d]", num)),
			c(bold+green, p.PackageAttrName),
			c(dim, nvl(p.PackageVersion, "?")),
		)
		fmt.Printf("     %s\n", highlight(nvl(p.PackageDescription, "-"), opts.Query))

		if num == 1 {
			fmt.Printf("     %s\n", c(dim, "nix-env -iA nixpkgs."+p.PackageAttrName))
		}
	}

	printSummary(count, opts)
}

func renderResultsVerbose(hits []ESHit, opts RenderOpts) {
	count := len(hits)

	// Reverse: best match last (bottom of terminal), numbered [1] = best
	for i := count - 1; i >= 0; i-- {
		p := hits[i].Source
		num := i + 1

		fmt.Println(c(dim, "───"))
		fmt.Printf("%s  %s  %s\n",
			c(cyan, fmt.Sprintf("[%d]", num)),
			c(bold+green, p.PackageAttrName),
			c(dim, nvl(p.PackageVersion, "?")),
		)
		fmt.Printf("     %s\n", highlight(nvl(p.PackageDescription, "-"), opts.Query))

		if len(p.PackagePrograms) > 0 {
			fmt.Printf("     %s %s\n",
				c(magenta, "programs"),
				strings.Join(p.PackagePrograms, "  "),
			)
		}

		if hp := homepage(p); hp != "" {
			fmt.Printf("     %s     %s\n", c(yellow, "home"), hp)
		}

		if lic := licenses(p); lic != "" {
			fmt.Printf("     %s  %s\n", c(yellow, "license"), lic)
		}

		if num == 1 {
			fmt.Printf("     %s\n", c(dim, "nix-env -iA nixpkgs."+p.PackageAttrName))
		}
	}

	printSummary(count, opts)
}

func printSummary(count int, opts RenderOpts) {
	fmt.Println()
	fmt.Println(c(dim, fmt.Sprintf("channel: %s  query: '%s'  showing %d of %d results  %dms",
		opts.Channel, opts.Query, count, opts.Total, opts.Elapsed.Milliseconds())))
}

func nvl(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func homepage(p SearchResult) string {
	switch v := p.PackageHomepage.(type) {
	case []interface{}:
		if len(v) > 0 {
			if s, ok := v[0].(string); ok {
				return s
			}
		}
	case string:
		return v
	}
	return ""
}

func licenses(p SearchResult) string {
	if len(p.PackageLicenseSet) == 0 {
		return ""
	}
	var parts []string
	for _, l := range p.PackageLicenseSet {
		switch {
		case l.Raw != "":
			parts = append(parts, l.Raw)
		case l.SpdxID != "":
			parts = append(parts, l.SpdxID)
		case l.FullName != "":
			parts = append(parts, l.FullName)
		}
	}
	return strings.Join(parts, ", ")
}
