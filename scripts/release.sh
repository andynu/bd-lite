#!/bin/sh
# Publish the tagged HEAD as a GitHub release with a linux/amd64 binary.
# Pushing the tag is deliberately left to the caller; this script only
# publishes a tag that is already on origin.
set -eu

die() {
	echo "release: $*" >&2
	exit 1
}

cd "$(git rev-parse --show-toplevel)"

[ -z "$(git status --porcelain)" ] || die "working tree is dirty"

tag=$(git describe --exact-match --tags --match 'v[0-9]*.[0-9]*.[0-9]*' HEAD 2>/dev/null) ||
	die "HEAD is not a vX.Y.Z tag"

git ls-remote --exit-code --tags origin "refs/tags/$tag" >/dev/null ||
	die "$tag is not on origin; push it first: git push origin $tag"

mkdir -p dist
GOOS=linux GOARCH=amd64 go build -o dist/bd-linux-amd64 .

built=$(dist/bd-linux-amd64 --version)
[ "$built" = "bd version $tag" ] || die "binary reports '$built', expected 'bd version $tag'"

version=${tag#v}
if grep -q "^## \[$version\].*Unreleased" CHANGELOG.md; then
	die "CHANGELOG.md still marks $version Unreleased; date it before releasing"
fi
awk -v hdr="## [$version]" '
	index($0, hdr) == 1 { on = 1; next }
	on && /^## \[/ { exit }
	on { print }
' CHANGELOG.md >dist/notes.md
grep -q '[^[:space:]]' dist/notes.md || die "no CHANGELOG.md section for $version"

gh release create "$tag" dist/bd-linux-amd64 --title "$tag" --notes-file dist/notes.md --verify-tag
