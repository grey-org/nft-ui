#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: scripts/finish-feature.sh -m "commit message" [--dry-run]

Commits the currently staged changes on dev, computes the next alpha tag from
existing v* tags, then pushes the branch and tag.

Use --all to stage all tracked and untracked changes first.
EOF
}

commit_message=""
dry_run=false
stage_all=false

while (($# > 0)); do
  case "$1" in
    -m|--message)
      shift
      if (($# == 0)); then
        printf 'Missing value for %s\n' '--message' >&2
        exit 1
      fi
      commit_message="$1"
      ;;
    --dry-run)
      dry_run=true
      ;;
    --all)
      stage_all=true
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      printf 'Unknown argument: %s\n\n' "$1" >&2
      usage >&2
      exit 1
      ;;
  esac
  shift
done

if [[ -z "$commit_message" ]]; then
  usage >&2
  exit 1
fi

branch=$(git rev-parse --abbrev-ref HEAD)
if [[ "$branch" != "dev" ]]; then
  printf 'Expected branch dev, got %s\n' "$branch" >&2
  exit 1
fi

index_env=()
tmp_index=""

cleanup() {
  if [[ -n "$tmp_index" && -f "$tmp_index" ]]; then
    rm -f "$tmp_index"
  fi
}

trap cleanup EXIT

if [[ "$dry_run" == true && "$stage_all" == true ]]; then
  git_index=$(git rev-parse --git-path index)
  tmp_index=$(mktemp)
  cp "$git_index" "$tmp_index"
  index_env=(env GIT_INDEX_FILE="$tmp_index")
fi

if [[ "$stage_all" == true ]]; then
  "${index_env[@]}" git add -A
fi

if "${index_env[@]}" git diff --cached --quiet; then
  printf 'No changes to commit.\n' >&2
  exit 1
fi

while IFS= read -r file; do
  case "$file" in
    .env|.env.*|config.yaml|config.yml|*.pem|*.key|*credentials*.json)
      printf 'Refusing to commit possible secret file: %s\n' "$file" >&2
      exit 1
      ;;
  esac
done < <("${index_env[@]}" git diff --cached --name-only)

latest_tag=""
major=0
minor=0
patch=0

while IFS= read -r tag; do
  if [[ "$tag" =~ ^v([0-9]+)\.([0-9]+)\.([0-9]+)(-.+)?$ ]]; then
    latest_tag="$tag"
    major=${BASH_REMATCH[1]}
    minor=${BASH_REMATCH[2]}
    patch=${BASH_REMATCH[3]}
    break
  fi
done < <(git tag --list 'v*' --sort=-version:refname)

next_tag="v${major}.${minor}.$((patch + 1))-alpha"

if git rev-parse "$next_tag" >/dev/null 2>&1; then
  printf 'Tag already exists: %s\n' "$next_tag" >&2
  exit 1
fi

printf 'Branch: %s\n' "$branch"
printf 'Latest tag: %s\n' "${latest_tag:-<none>}"
printf 'Next tag: %s\n' "$next_tag"

if [[ "$dry_run" == true ]]; then
  printf '[dry-run] would create the commit, tag, and pushes.\n'
  exit 0
fi

git commit -m "$commit_message"
git tag -a "$next_tag" -m "$commit_message"
git push origin "$branch"
git push origin "$next_tag"

printf 'Pushed %s and %s\n' "$branch" "$next_tag"
