#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILL_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
FILES_DIR="${SKILL_ROOT}/files"

FORCE=0
ARGS=()
for arg in "$@"; do
  if [[ "$arg" == "--force" ]]; then
    FORCE=1
  else
    ARGS+=("$arg")
  fi
done

REQUESTED_MODULE_NAME="${ARGS[0]:-}"
MODULE_NAME="my"
DEVER_VERSION="${ARGS[1]:-main}"
APP_NAME="${ARGS[2]:-dever-app}"
PORT="${ARGS[3]:-8082}"

if [[ -z "$REQUESTED_MODULE_NAME" ]]; then
  echo "Usage: bash scripts/boot.sh <module_name> [dever_version] [app_name] [port] [--force]"
  echo "Note: Dever application projects always use Go module path: my"
  exit 1
fi

if [[ "$REQUESTED_MODULE_NAME" != "$MODULE_NAME" ]]; then
  echo "Ignoring requested module path: $REQUESTED_MODULE_NAME"
  echo "Dever application projects always use Go module path: $MODULE_NAME"
fi

if [[ ! -f go.mod ]]; then
  go mod init "$MODULE_NAME"
else
  EXISTING_MODULE="$(awk '/^module /{print $2; exit}' go.mod)"
  if [[ -n "$EXISTING_MODULE" && "$EXISTING_MODULE" != "$MODULE_NAME" ]]; then
    echo "Detected go.mod module path: $EXISTING_MODULE"
    echo "Dever package components require module path: $MODULE_NAME"
    echo "Refuse to continue instead of generating incompatible imports."
    exit 1
  fi
fi

copy_file() {
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
  cp "$src" "$dest"
}

render_template() {
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
    -e "s/{{MODULE_NAME}}/${MODULE_NAME}/g" \
    -e "s/{{APP_NAME}}/${APP_NAME}/g" \
    -e "s/{{PORT}}/${PORT}/g" \
    "$src" > "$dest"
}

ensure_gitignore() {
  local file=".gitignore"
  local template="${FILES_DIR}/gitignore"
  local marker="# >>> dever generated ignore"
  if [[ ! -f "$template" ]]; then
    echo "Missing gitignore template: $template"
    exit 1
  fi
  if [[ ! -f "$file" ]]; then
    cp "$template" "$file"
    return
  fi
  if grep -Fq "$marker" "$file"; then
    return
  fi
  [[ -s "$file" ]] && printf '\n' >> "$file"
  cat "$template" >> "$file"
}

go get "github.com/shemic/dever@${DEVER_VERSION}"

mkdir -p config/front/assets/{admin,work}/images middleware data/{load,log} package module/main/model
ensure_gitignore

render_template "${FILES_DIR}/go/main.go.tmpl" "main.go"
render_template "${FILES_DIR}/config/setting.jsonc.tmpl" "config/setting.jsonc"
copy_file "${FILES_DIR}/config/front.jsonc.tmpl" "config/front.jsonc"
copy_file "${FILES_DIR}/go/middleware/readme.txt" "middleware/readme.txt"
copy_file "${FILES_DIR}/go/data/readme.txt" "data/readme.txt"
copy_file "${FILES_DIR}/go/package/readme.txt" "package/readme.txt"

for site in admin work; do
  for file in logo.svg favicon.svg; do
    copy_file "${FILES_DIR}/config/front/assets/${site}/images/${file}" "config/front/assets/${site}/images/${file}"
  done
done

echo "Generated minimal Dever project skeleton."
echo "No business API or Service was generated."
