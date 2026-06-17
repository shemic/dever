#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILL_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
TEMPLATE_DIR="${SKILL_ROOT}/files/component"

OWNER_KIND="${1:-}"
COMPONENT_RAW="${2:-}"
COMPONENT_TITLE="${3:-}"
FORCE=0

shift $(( $# >= 3 ? 3 : $# )) || true
for arg in "$@"; do
  case "$arg" in
    --force) FORCE=1 ;;
    *) echo "Unknown option: $arg"; exit 1 ;;
  esac
done

if [[ -z "$OWNER_KIND" || -z "$COMPONENT_RAW" ]]; then
  echo "Usage: bash scripts/component-skill.sh <package|module> <component_name> [component_title] [--force]"
  exit 1
fi

case "$OWNER_KIND" in
  package|module) ;;
  *) echo "owner kind must be package or module"; exit 1 ;;
esac

if [[ ! "$COMPONENT_RAW" =~ ^[A-Za-z0-9_-]+$ ]]; then
  echo "component_name only supports letters, numbers, underscore and hyphen."
  exit 1
fi

COMPONENT_NAME="$(echo "$COMPONENT_RAW" | tr '[:upper:]' '[:lower:]' | tr '-' '_')"
COMPONENT_TITLE="${COMPONENT_TITLE:-$COMPONENT_NAME}"
BASE_DIR="${OWNER_KIND}/${COMPONENT_NAME}"
SKILL_DIR="${BASE_DIR}/skills/${COMPONENT_NAME}"

render() {
  local src="$1"
  local dest="$2"
  if [[ ! -f "$src" ]]; then
    echo "Missing template: $src"
    exit 1
  fi
  if [[ -e "$dest" && "$FORCE" != "1" ]]; then
    return
  fi
  if [[ -e "$dest" && "$FORCE" == "1" ]]; then
    cp "$dest" "${dest}.bak"
  fi
  mkdir -p "$(dirname "$dest")"
  sed \
    -e "s/{{COMPONENT_NAME}}/${COMPONENT_NAME}/g" \
    -e "s/{{COMPONENT_TITLE}}/${COMPONENT_TITLE}/g" \
    "$src" > "$dest"
}

render "${TEMPLATE_DIR}/dever.json.tmpl" "${BASE_DIR}/dever.json"
render "${TEMPLATE_DIR}/skills/SKILL.md.tmpl" "${SKILL_DIR}/SKILL.md"
render "${TEMPLATE_DIR}/skills/README.md.tmpl" "${SKILL_DIR}/README.md"

echo "Generated component skill skeleton:"
echo "  ${BASE_DIR}/dever.json"
echo "  ${SKILL_DIR}/SKILL.md"
echo "  ${SKILL_DIR}/README.md"
