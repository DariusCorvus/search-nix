# search-nix

Search NixOS packages from the terminal. Queries the [search.nixos.org](https://search.nixos.org) Elasticsearch backend directly.

## Install

```sh
# With Nix flakes
nix profile install github:DariusCorvus/search-nix

# Or build from source
git clone https://github.com/DariusCorvus/search-nix.git
cd search-nix
go build -o search-nix .
```

## Usage

```
Usage: search-nix [OPTIONS] [query]

Options:
  -c, --channel <channel>   Channel to search (unstable, 25.11, 24.11, ...)
                            Default: auto-detected from nixos-version
  -n, --size <n>            Number of results (default: 20, max: 50)
  -v, --verbose             Show full output (homepage, license, programs)
  -t, --tui                 Launch interactive TUI mode
  -h, --help                Show this help
```

### CLI mode

```sh
search-nix fuser              # search for "fuser"
search-nix -v fuser           # verbose output (homepage, license, all programs)
search-nix -n 5 python linter # top 5 results for "python linter"
search-nix -c unstable ffmpeg # search the unstable channel
```

Results are displayed with the best match at the bottom of the terminal for easy reading:

```
───
[2]  busybox  1.37.0
     Tiny versions of common UNIX utilities in a single small executable
     programs fuser  runsv  swapon  openvt  reset  (+397 more)
───
[1]  psmisc  23.7
     Set of small useful utilities that use the proc filesystem (such as fuser, killall and pstree)
     programs fuser  peekfd  pstree.x11  killall  pstree  (+2 more)
     nix-env -iA nixpkgs.psmisc

channel: 25.11  query: 'fuser'  showing 5 of 142 results  38ms
```

### TUI mode

Launch an interactive full-screen interface:

```sh
search-nix -t              # start with empty input
search-nix -t fuser        # start with pre-filled query and immediate search
```

#### Keybindings

| Key | Action |
|-----|--------|
| Type + `Enter` | Search for the query |
| `j` / `Down` | Move cursor down |
| `k` / `Up` | Move cursor up |
| `Enter` (on result) | Open detail view |
| `/` or `Tab` | Refocus search input |
| `Esc` | Clear input / go back |
| `q` | Quit (from results/detail) |
| `Ctrl+C` | Quit (always) |

The detail view shows the full package info: description, all programs, homepage, license, and install command.

## Channel detection

If no channel is specified with `-c`, `search-nix` auto-detects from `nixos-version`. Falls back to `unstable` if detection fails or the detected channel doesn't have an ES index.

## License

MIT
