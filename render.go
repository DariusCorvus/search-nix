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
	noul    = "\033[24m"
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
	// Build alternation of all query terms so each word highlights independently
	terms := strings.Fields(query)
	var escaped []string
	for _, t := range terms {
		escaped = append(escaped, regexp.QuoteMeta(t))
	}
	re, err := regexp.Compile("(?i)" + strings.Join(escaped, "|"))
	if err != nil {
		return s
	}
	return re.ReplaceAllStringFunc(s, func(match string) string {
		return bold + ul + match + reset
	})
}

// highlightInline underlines matching terms without resetting surrounding styles.
func highlightInline(s string, query string) string {
	if !hasColor || query == "" {
		return s
	}
	terms := strings.Fields(query)
	var escaped []string
	for _, t := range terms {
		escaped = append(escaped, regexp.QuoteMeta(t))
	}
	re, err := regexp.Compile("(?i)" + strings.Join(escaped, "|"))
	if err != nil {
		return s
	}
	return re.ReplaceAllStringFunc(s, func(match string) string {
		return ul + match + noul
	})
}

func renderResults(hits []ESHit, opts RenderOpts) {
	count := len(hits)

	// Reverse: best match last (bottom of terminal), numbered [1] = best
	for i := count - 1; i >= 0; i-- {
		p := hits[i].Source
		num := i + 1

		fmt.Println(c(dim, "───"))

		matchTag := ""
		if hasExactMatch(p, opts.Query) {
			matchTag = " " + c(bold+green, "*")
		}

		fmt.Printf("%s  %s  %s%s\n",
			c(cyan, fmt.Sprintf("[%d]", num)),
			c(bold+green, highlightInline(p.PackageAttrName, opts.Query)),
			c(dim, nvl(p.PackageVersion, "?")),
			matchTag,
		)
		fmt.Printf("     %s\n", highlight(nvl(p.PackageDescription, "-"), opts.Query))

		if len(p.PackagePrograms) > 0 {
			fmt.Printf("     %s %s\n",
				c(magenta, "programs"),
				peekPrograms(p.PackagePrograms, 5, opts.Query),
			)
		}

		if num == 1 {
			if hp := homepage(p); hp != "" {
				fmt.Printf("     %s     %s\n", c(yellow, "home"), hp)
			}
			fmt.Printf("     %s\n", c(dim, "nix profile install nixpkgs#"+p.PackageAttrName))
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

		matchTag := ""
		if hasExactMatch(p, opts.Query) {
			matchTag = " " + c(bold+green, "*")
		}

		fmt.Printf("%s  %s  %s%s\n",
			c(cyan, fmt.Sprintf("[%d]", num)),
			c(bold+green, highlightInline(p.PackageAttrName, opts.Query)),
			c(dim, nvl(p.PackageVersion, "?")),
			matchTag,
		)
		fmt.Printf("     %s\n", highlight(nvl(p.PackageDescription, "-"), opts.Query))

		if ld := longDesc(p); ld != "" {
			fmt.Printf("     %s\n", c(dim, ld))
		}

		if len(p.PackagePrograms) > 0 {
			fmt.Printf("     %s %s\n",
				c(magenta, "programs"),
				peekPrograms(p.PackagePrograms, 10, opts.Query),
			)
		}

		if hp := homepage(p); hp != "" {
			fmt.Printf("     %s     %s\n", c(yellow, "home"), hp)
		}

		if lic := licenses(p); lic != "" {
			fmt.Printf("     %s  %s\n", c(yellow, "license"), lic)
		}

		if num == 1 {
			fmt.Printf("     %s\n", c(dim, "nix profile install nixpkgs#"+p.PackageAttrName))
			fmt.Printf("     %s\n", c(dim, "nix-env -iA nixpkgs."+p.PackageAttrName))
		}
	}

	printSummary(count, opts)
}

func printSummary(count int, opts RenderOpts) {
	fmt.Println()
	fmt.Println(c(dim, fmt.Sprintf("channel: %s | query: '%s' | showing %d of %d | %dms",
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

// stripHTML removes HTML tags and cleans up a long description for terminal display.
func stripHTML(s string) string {
	// Remove <rendered-html> wrapper
	s = strings.TrimPrefix(s, "<rendered-html>")
	s = strings.TrimSuffix(s, "</rendered-html>")
	// Strip all HTML tags
	re := regexp.MustCompile("<[^>]*>")
	s = re.ReplaceAllString(s, "")
	// Collapse whitespace
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimSpace(s)
}

func longDesc(p SearchResult) string {
	if p.PackageLongDescription == "" {
		return ""
	}
	s := stripHTML(p.PackageLongDescription)
	if s == p.PackageDescription {
		return ""
	}
	return s
}

func peekPrograms(progs []string, max int, query string) string {
	// Partition: matching programs first, then the rest
	var matched, rest []string
	terms := strings.Fields(strings.ToLower(query))
	for _, p := range progs {
		if matchesAnyTerm(p, terms) {
			matched = append(matched, highlight(p, query))
		} else {
			rest = append(rest, p)
		}
	}
	ordered := append(matched, rest...)
	if len(ordered) <= max {
		return strings.Join(ordered, "  ")
	}
	return strings.Join(ordered[:max], "  ") + fmt.Sprintf("  (+%d more)", len(ordered)-max)
}

func hasExactMatch(p SearchResult, query string) bool {
	if query == "" {
		return false
	}
	q := strings.ToLower(query)
	// Check package name and attr name
	if strings.ToLower(p.PackagePname) == q || strings.ToLower(p.PackageAttrName) == q {
		return true
	}
	// Check programs
	for _, prog := range p.PackagePrograms {
		if strings.ToLower(prog) == q {
			return true
		}
	}
	return false
}

func matchesAnyTerm(prog string, terms []string) bool {
	lower := strings.ToLower(prog)
	for _, t := range terms {
		if strings.Contains(lower, t) {
			return true
		}
	}
	return false
}
