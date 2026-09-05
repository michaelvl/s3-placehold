#!/usr/bin/env bash
#
# Regenerates the images in examples/ from README.md.
#
# The gallery at the end of README.md shows a curl command above every image.
# This script extracts those commands and runs them verbatim, so the gallery
# cannot drift from what the server actually produces: each command names its
# own output file with `-o examples/<name>.<ext>`, and that is the file the
# README displays.
#
# Unless a server already answers on the port, one is built and started here.

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

readme="README.md"
out_dir="examples"
port="${PORT:-9000}"
buckets="${BUCKETS:-images:public}"
base_url="http://localhost:${port}"

# Extracts the `curl ...` lines from fenced code blocks in README's gallery
# section, i.e. from the "## Gallery" heading to the end of the file.
extract_commands() {
	awk '
		/^## Gallery[[:space:]]*$/ { in_gallery = 1; next }
		in_gallery && /^## / { in_gallery = 0 }
		in_gallery && /^```/ { in_fence = !in_fence; next }
		in_gallery && in_fence && /^curl / { print }
	' "$readme"
}

server_is_up() {
	curl -fsS -o /dev/null "${base_url}/images/format=png/size=1x1" 2>/dev/null
}

# The gallery commands are shown without curl's `-f`, so a rejected key lands in
# the file as an XML error rather than failing the command. Check that what was
# written is really an image of the format the file name claims.
is_image() {
	local file="$1"
	case "${file##*.}" in
	png) [[ "$(head -c 8 "$file" | od -An -tx1 | tr -d ' \n')" == "89504e470d0a1a0a" ]] ;;
	jpg | jpeg) [[ "$(head -c 2 "$file" | od -An -tx1 | tr -d ' \n')" == "ffd8" ]] ;;
	svg) head -c 1024 "$file" | grep -q '<svg' ;;
	*)
		echo "error: unsupported example file type: $file" >&2
		exit 1
		;;
	esac
}

mapfile -t commands < <(extract_commands)

if [[ ${#commands[@]} -eq 0 ]]; then
	echo "error: no curl commands found in the '## Gallery' section of $readme" >&2
	exit 1
fi

# Every gallery command must write into examples/, or there is nothing to show
# in the README next to it.
declare -a expected_files=()
for cmd in "${commands[@]}"; do
	if [[ ! "$cmd" =~ -o[[:space:]]+(${out_dir}/[A-Za-z0-9._-]+) ]]; then
		echo "error: gallery command does not write to ${out_dir}/: $cmd" >&2
		exit 1
	fi
	expected_files+=("${BASH_REMATCH[1]}")
done

server_pid=""
curl_stderr="$(mktemp)"
cleanup() {
	rm -f "$curl_stderr"
	if [[ -n "$server_pid" ]]; then
		kill "$server_pid" 2>/dev/null || true
		wait "$server_pid" 2>/dev/null || true
	fi
}
trap cleanup EXIT

if server_is_up; then
	echo "using the server already listening on ${base_url}"
else
	echo "starting a server on ${base_url} (BUCKETS=${buckets})"
	go build -o bin/s3-placehold ./cmd/s3-placehold
	BUCKETS="$buckets" PORT="$port" ./bin/s3-placehold >/dev/null 2>&1 &
	server_pid=$!

	for _ in $(seq 50); do
		server_is_up && break
		if ! kill -0 "$server_pid" 2>/dev/null; then
			echo "error: server exited during startup" >&2
			exit 1
		fi
		sleep 0.1
	done

	if ! server_is_up; then
		echo "error: server did not become ready on ${base_url}" >&2
		exit 1
	fi
fi

mkdir -p "$out_dir"

for i in "${!commands[@]}"; do
	cmd="${commands[$i]}"
	file="${expected_files[$i]}"
	echo "  ${file}"
	if ! eval "$cmd" 2>"$curl_stderr"; then
		cat "$curl_stderr" >&2
		echo "error: command failed: $cmd" >&2
		exit 1
	fi
	if [[ ! -s "$file" ]] || ! is_image "$file"; then
		echo "error: command did not return an image, the server rejected the key: $cmd" >&2
		head -c 400 "$file" >&2
		echo >&2
		exit 1
	fi
done

# Files nobody asked for are stale renders from an example that was edited or
# dropped; leaving them behind makes examples/ grow silently.
while IFS= read -r found; do
	for file in "${expected_files[@]}"; do
		[[ "$found" == "$file" ]] && continue 2
	done
	echo "removing stale ${found}"
	rm -f "$found"
done < <(find "$out_dir" -type f -not -name '.*' | sort)

echo "generated ${#commands[@]} example(s) in ${out_dir}/"
