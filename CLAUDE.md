# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

Single-file bash CLI tool (`search-nix.sh`) that searches NixOS packages by querying the search.nixos.org Elasticsearch backend directly. Intended to be installed as `nix-search`.

## How It Works

1. Parses CLI args (`-c`/`--channel`, `-n`/`--size`, positional query terms)
2. Auto-detects the NixOS channel from `nixos-version` if not specified, with a fallback probe to verify the ES index exists
3. Builds an Elasticsearch `dis_max` query (cross_fields on package attrs/names/descriptions + best_fields with fuzziness on programs)
4. POSTs to `search.nixos.org/backend/latest-{schema}-nixos-{channel}/_search` with basic auth
5. Formats results via `jq` showing package name, version, description, programs, homepage, and license

## Dependencies

Runtime: `bash`, `curl`, `jq`, `grep` (with `-P` / PCRE support)

## Testing

No test suite. Test manually:

```sh
./search-nix.sh fuser
./search-nix.sh -n 5 -c unstable python linter
```

## Key Details

- ES credentials and schema version are hardcoded at the top of the script
- `set -euo pipefail` is used — unset variables and failed commands will abort
- Max result size is documented as 50 in the help text but not enforced in code
