#!/usr/bin/env bash
set -euo pipefail

# ── config ────────────────────────────────────────────────────────────────────
ES_USER="aWVSALXpZv"
ES_PASS="X8gPHnzL52wFEekuxsfQ9cSh"
ES_SCHEMA="44"
SIZE=20
CHANNEL=""

# ── usage ─────────────────────────────────────────────────────────────────────
usage() {
  cat <<EOF
Usage: nix-search [OPTIONS] <query>

Options:
  -c, --channel <channel>   Channel to search (unstable, 25.11, 24.11, ...)
                            Default: auto-detected from nixos-version
  -n, --size <n>            Number of results (default: 20, max: 50)
  -h, --help                Show this help

Examples:
  nix-search fuser
  nix-search -n 5 python linter
  nix-search -c unstable ffmpeg
EOF
  exit 0
}

# ── args ──────────────────────────────────────────────────────────────────────
POSITIONAL=()
while [[ $# -gt 0 ]]; do
  case $1 in
    -c|--channel) CHANNEL="$2"; shift 2 ;;
    -n|--size)    SIZE="$2";    shift 2 ;;
    -h|--help)    usage ;;
    *) POSITIONAL+=("$1"); shift ;;
  esac
done

QUERY="${POSITIONAL[*]:-}"

if [[ -z "$QUERY" ]]; then
  echo "error: no search query provided" >&2
  echo "Usage: nix-search <query>" >&2
  exit 1
fi

# ── channel detection ─────────────────────────────────────────────────────────
if [[ -z "$CHANNEL" ]]; then
  if command -v nixos-version &>/dev/null; then
    VERSION=$(nixos-version 2>/dev/null || true)
    if echo "$VERSION" | grep -qP 'pre|unstable'; then
      CHANNEL="unstable"
    else
      DETECTED=$(echo "$VERSION" | grep -oP '^\d+\.\d+' || true)
      if [[ -z "$DETECTED" ]]; then
        CHANNEL="unstable"
      else
        # check if the index actually exists before committing to it
        HTTP=$(curl -s -o /dev/null -w "%{http_code}" \
          -u "${ES_USER}:${ES_PASS}" \
          "https://search.nixos.org/backend/latest-${ES_SCHEMA}-nixos-${DETECTED}")
        if [[ "$HTTP" == "200" ]]; then
          CHANNEL="$DETECTED"
        else
          CHANNEL="unstable"
        fi
      fi
    fi
  else
    CHANNEL="unstable"
  fi
fi

BACKEND="https://search.nixos.org/backend/latest-${ES_SCHEMA}-nixos-${CHANNEL}/_search"

# ── query ─────────────────────────────────────────────────────────────────────
PAYLOAD=$(jq -nc \
  --arg q "$QUERY" \
  --argjson size "$SIZE" \
  '{
    from: 0,
    size: $size,
    query: {
      bool: {
        filter: [{ term: { type: { value: "package" } } }],
        must: [{
          dis_max: {
            tie_breaker: 0.7,
            queries: [
              {
                multi_match: {
                  type: "cross_fields",
                  query: $q,
                  analyzer: "whitespace",
                  operator: "and",
                  fields: [
                    "package_attr_name^9",
                    "package_attr_name.*^5.4",
                    "package_pname^6",
                    "package_pname.*^3.6",
                    "package_description^1.3",
                    "package_longDescription^1"
                  ]
                }
              },
              {
                multi_match: {
                  type: "best_fields",
                  query: $q,
                  analyzer: "whitespace",
                  operator: "and",
                  fields: ["package_programs^7.5"],
                  fuzziness: 1
                }
              }
            ]
          }
        }]
      }
    }
  }')

RESPONSE=$(curl -s \
  -u "${ES_USER}:${ES_PASS}" \
  -H "Content-Type: application/json" \
  -X POST "$BACKEND" \
  -d "$PAYLOAD")

# ── error check ───────────────────────────────────────────────────────────────
if echo "$RESPONSE" | jq -e '.error' &>/dev/null; then
  echo "error: $(echo "$RESPONSE" | jq -r '.error.reason')" >&2
  exit 1
fi

TOTAL=$(echo "$RESPONSE" | jq -r '.hits.total.value')

if [[ "$TOTAL" -eq 0 ]]; then
  echo "no results for '${QUERY}' on channel ${CHANNEL}"
  exit 0
fi

# ── output ────────────────────────────────────────────────────────────────────
echo "channel: ${CHANNEL}  query: '${QUERY}'  showing $(echo "$RESPONSE" | jq '.hits.hits | length') of ${TOTAL} results"
echo

echo "$RESPONSE" | jq -r '
  [.hits.hits[]._source] | reverse | .[] |
  [
    "pkg      \(.package_attr_name)  \(.package_version // "?")",
    "desc     \(.package_description // "-")",
    if (.package_programs | length) > 0
      then "programs \(.package_programs | join("  "))"
      else empty
    end,
    if (.package_homepage | type) == "array"
      then "home     \(.package_homepage[0])"
    elif .package_homepage
      then "home     \(.package_homepage)"
      else empty
    end,
    if (.package_license_set | length) > 0
      then "license  \([.package_license_set[] | if type == "object" then (.spdxId // .fullName) else . end] | map(select(. != null)) | join(", "))"
      else empty
    end,
    ""
  ] | .[]
'
