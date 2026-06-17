#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILL_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
TEMPLATE_DIR="${SKILL_ROOT}/files/page/standard"

OWNER_DIR="${1:-}"
SITE_KEY="${2:-}"
RESOURCE_RAW="${3:-}"
PAGE_KIND="${4:-}"
RESOURCE_TITLE="${5:-}"
FORCE=0

shift $(( $# >= 5 ? 5 : $# )) || true
for arg in "$@"; do
  case "$arg" in
    --force) FORCE=1 ;;
    *) echo "Unknown option: $arg"; exit 1 ;;
  esac
done

if [[ -z "$OWNER_DIR" || -z "$SITE_KEY" || -z "$RESOURCE_RAW" || -z "$PAGE_KIND" ]]; then
  echo "Usage: bash scripts/page.sh <module-or-package-dir> <site_key> <resource_name> <list|update|detail> [resource_title] [--force]"
  echo "Example: bash scripts/page.sh module/demo admin product list 产品"
  exit 1
fi

case "$PAGE_KIND" in
  list|update|detail) ;;
  *) echo "Unsupported page kind: $PAGE_KIND"; exit 1 ;;
esac

if [[ ! "$SITE_KEY" =~ ^[A-Za-z0-9_-]+$ || ! "$RESOURCE_RAW" =~ ^[A-Za-z0-9_-]+$ ]]; then
  echo "site_key and resource_name only support letters, numbers, underscore and hyphen."
  exit 1
fi

RESOURCE_NAME="$(echo "$RESOURCE_RAW" | tr '[:upper:]' '[:lower:]' | tr '-' '_')"
RESOURCE_TITLE="${RESOURCE_TITLE:-$RESOURCE_NAME}"
TEMPLATE="${TEMPLATE_DIR}/${PAGE_KIND}.json.tmpl"
TARGET="${OWNER_DIR}/front/page/${SITE_KEY}/${RESOURCE_NAME}/${PAGE_KIND}.json"

if [[ ! -f "$TEMPLATE" ]]; then
  echo "Missing template: $TEMPLATE"
  exit 1
fi
if [[ -e "$TARGET" && "$FORCE" != "1" ]]; then
  echo "Refuse to overwrite existing file: $TARGET"
  echo "Re-run with --force only after confirming replacement."
  exit 1
fi
if [[ -e "$TARGET" && "$FORCE" == "1" ]]; then
  cp "$TARGET" "${TARGET}.bak"
fi

mkdir -p "$(dirname "$TARGET")"
sed \
  -e "s/{{RESOURCE_NAME}}/${RESOURCE_NAME}/g" \
  -e "s/{{RESOURCE_TITLE}}/${RESOURCE_TITLE}/g" \
  "$TEMPLATE" > "$TARGET"

echo "Generated:"
echo "  $TARGET"
echo "Standard page skeleton uses model inference; no Service/API was generated."
