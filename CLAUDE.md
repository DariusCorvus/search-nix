# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

Go CLI tool that searches NixOS packages by querying the search.nixos.org Elasticsearch backend directly. Installed as `search-nix`.

## How It Works

1. Parses CLI args (`-c`/`--channel`, `-n`/`--size`, `-v`/`--verbose`, `-t`/`--tui`, positional query terms)
2. Auto-detects the NixOS channel from `nixos-version` if not specified, with a fallback probe to verify the ES index exists
3. Builds an Elasticsearch `dis_max` query (cross_fields on package attrs/names/descriptions + best_fields with fuzziness on programs)
4. POSTs to `search.nixos.org/backend/latest-{schema}-nixos-{channel}/_search` with basic auth
5. Formats results with colored terminal output showing package name, version, description, programs, homepage, and license

## Project Structure

- `main.go` — entrypoint, arg parsing, orchestration
- `search.go` — ES query building, HTTP request, response parsing
- `render.go` — compact/verbose terminal output, colors, highlighting
- `tui.go` — interactive TUI mode using bubbletea (launched with `-t`/`--tui`)
- `channel.go` — channel auto-detection (nixos-version)
- `flake.nix` — Nix flake for building and dev shell

## Dependencies

Build: Go compiler. External deps: `golang.org/x/term` (tty detection), `charmbracelet/bubbletea` + `bubbles` + `lipgloss` (TUI). No runtime dependencies — produces a static binary.

Dev shell via `nix develop` provides: `go`, `gopls`, `gotools`.

## Building

```sh
# With Go directly
go build -o search-nix .

# With Nix
nix build
```

## Testing

No test suite. Test manually:

```sh
./search-nix fuser
./search-nix -v fuser
./search-nix -n 5 -c unstable python linter
./search-nix fuser | cat   # verify no colors when piped
./search-nix -t              # TUI mode, empty input
./search-nix -t fuser        # TUI mode with pre-filled query
```

## Key Details

- ES credentials and schema version are hardcoded as constants in `search.go`
- Results are displayed reversed (best match `[1]` at bottom of terminal)
- Colors auto-disable when stdout is not a tty
- `search-nix.sh` is the original bash implementation, kept for reference
- TUI mode (`-t`/`--tui`): interactive bubbletea interface with text input, scrollable results list, and detail view (Enter on a result). Keys: `j/k`/arrows to navigate, `Enter` to search or view detail, `Esc`/`backspace` to go back, `/`/`Tab` to refocus input, `q` to quit
