# search-nix

Search NixOS packages from the terminal. Queries the [search.nixos.org](https://search.nixos.org) Elasticsearch backend directly.

## Install

```sh
# Run directly without installing
nix run github:DariusCorvus/search-nix -- fuser

# Install to profile
nix profile install github:DariusCorvus/search-nix
```

Or add as a flake input:

```nix
{
  inputs.search-nix.url = "github:DariusCorvus/search-nix";

  # then use search-nix.packages.${system}.default
}
```

Or build from source:

```sh
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
[2]  busybox  1.37.0 *
     Tiny versions of common UNIX utilities in a single small executable
     programs fuser  runsv  swapon  openvt  reset  (+397 more)
───
[1]  psmisc  23.7 *
     Set of small useful utilities that use the proc filesystem (such as fuser, killall and pstree)
     programs fuser  peekfd  pstree.x11  killall  pstree  (+2 more)
     home     https://gitlab.com/psmisc/psmisc
     nix profile install nixpkgs#psmisc
     nix-env -iA nixpkgs.psmisc

channel: unstable | query: 'fuser' | showing 5 of 5 | 38ms
```

A `*` after the version indicates an exact program name match. The top result `[1]` shows the homepage and both install commands.

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
| `j` / `Down` | Move cursor down / next section |
| `k` / `Up` | Move cursor up / prev section |
| `Enter` (on result) | Toggle inline detail (programs, homepage, license, install command) |
| `Space` | Pin/unpin detail open (stays open when moving cursor) |
| `a` | Toggle expand/collapse all details |
| `Left` / `Right` | Switch channel (when channel bar is focused) |
| `c` | Jump to channel selector |
| `/` or `Tab` | Refocus search input |
| `y` | Copy package name |
| `e` | Copy nix-env install command |
| `p` | Copy nix profile install command |
| `r` | Open nix-shell with package |
| `o` | Open homepage in browser |
| `Esc` | Collapse detail / clear input / quit |
| `q` | Quit (from results) |
| `?` | Toggle help page |
| `Ctrl+C` | Quit (always) |

Navigation flows vertically through three sections: **channel bar** → **search input** → **results**. Press `Up` from the first result to focus the search input, and `Up` again to focus the channel bar where `Left`/`Right` switches channels.

Pressing `Enter` on a result expands its details inline. `Space` pins a result open so it stays expanded as you navigate. `a` toggles expand-all mode which persists across searches.

## Channel detection

If no channel is specified with `-c`, `search-nix` auto-detects from `nixos-version`. Falls back to `unstable` if detection fails or the detected channel doesn't have an ES index.

## License

MIT
