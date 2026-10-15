#!/bin/sh
# Install niimtui from the latest GitHub release.
#
#   curl -fsSL https://raw.githubusercontent.com/davidsanchez222/niimtui/main/install.sh | sh
#
# Options (after `sh -s --` when piped):
#   --try [args...]   run niimtui from a temp folder without installing it
#   --version <tag>   install a specific release, such as v0.1.0
#   --dir <path>      install into <path> instead of ~/.local/bin
#
# NIIMTUI_VERSION and NIIMTUI_INSTALL_DIR set the same values from the environment.

set -eu

REPO="davidsanchez222/niimtui"
version="${NIIMTUI_VERSION:-}"
install_dir="${NIIMTUI_INSTALL_DIR:-$HOME/.local/bin}"
try=0

say() { printf 'niimtui: %s\n' "$*" >&2; }
die() {
	say "error: $*"
	exit 1
}

while [ $# -gt 0 ]; do
	case "$1" in
	--try)
		try=1
		shift
		break
		;;
	--version)
		[ $# -ge 2 ] || die "--version needs a tag"
		version="$2"
		shift 2
		;;
	--dir)
		[ $# -ge 2 ] || die "--dir needs a path"
		install_dir="$2"
		shift 2
		;;
	-h | --help)
		cat <<'EOF'
usage: install.sh [--try [niimtui args...]] [--version <tag>] [--dir <path>]

  --try [args...]   run niimtui from a temp folder without installing it
  --version <tag>   install a specific release, such as v0.1.0
  --dir <path>      install into <path> instead of ~/.local/bin
EOF
		exit 0
		;;
	*) die "unknown option: $1" ;;
	esac
done

[ "$(uname -s)" = "Darwin" ] || die "only macOS builds are published so far; see https://github.com/$REPO#support"

case "$(uname -m)" in
arm64) arch=arm64 ;;
x86_64)
	# A shell running under Rosetta reports x86_64 on Apple Silicon.
	if [ "$(sysctl -n hw.optional.arm64 2>/dev/null || echo 0)" = "1" ]; then
		arch=arm64
	else
		arch=amd64
	fi
	;;
*) die "unsupported architecture: $(uname -m)" ;;
esac

command -v curl >/dev/null 2>&1 || die "curl is required"

if [ -z "$version" ]; then
	# The latest-release page redirects to the tag, which avoids the API rate limit.
	version=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$REPO/releases/latest")
	version="${version##*/}"
	case "$version" in
	v*) ;;
	*) die "could not find the latest release" ;;
	esac
fi
case "$version" in
v*) ;;
*) version="v$version" ;;
esac

archive="niimtui_${version#v}_darwin_${arch}.tar.gz"
base="https://github.com/$REPO/releases/download/$version"

tmp=$(mktemp -d "${TMPDIR:-/tmp}/niimtui.XXXXXX")
trap 'rm -rf "$tmp"' EXIT INT TERM

say "downloading $version ($arch)"
curl -fsSL -o "$tmp/$archive" "$base/$archive" || die "download failed: $base/$archive"
curl -fsSL -o "$tmp/checksums.txt" "$base/checksums.txt" || die "download failed: $base/checksums.txt"

expected=$(awk -v f="$archive" '$2 == f { print $1 }' "$tmp/checksums.txt")
[ -n "$expected" ] || die "$archive is missing from checksums.txt"
actual=$(shasum -a 256 "$tmp/$archive" | awk '{ print $1 }')
[ "$expected" = "$actual" ] || die "checksum mismatch for $archive"

tar -xzf "$tmp/$archive" -C "$tmp" niimtui

if [ "$try" = 1 ]; then
	# When piped into sh, stdin is the script, so give the TUI the terminal.
	if [ ! -t 0 ] && (: </dev/tty) 2>/dev/null; then
		"$tmp/niimtui" "$@" </dev/tty
	else
		"$tmp/niimtui" "$@"
	fi
	exit $?
fi

mkdir -p "$install_dir"
mv "$tmp/niimtui" "$install_dir/niimtui"
chmod 755 "$install_dir/niimtui"
say "installed $version to $install_dir/niimtui"

case ":$PATH:" in
*":$install_dir:"*) say "run: niimtui" ;;
*)
	say "$install_dir is not on your PATH. Add this to your shell profile:"
	printf '  export PATH="%s:$PATH"\n' "$install_dir" >&2
	;;
esac
