#!/usr/bin/env bash
# A disposable contributor vault/configuration and a binary built from this tree.
set -euo pipefail
repo=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
root=$(mktemp -d "${TMPDIR:-/tmp}/zn-sandbox.XXXXXX")
server_pid=
cleanup() {
  if [[ -n "$server_pid" ]]; then
    kill -INT "$server_pid" 2>/dev/null || true
    wait "$server_pid" 2>/dev/null || true
  fi
  rm -rf -- "$root"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

unset ZENNOTES_VAULT ZENNOTES_SERVER ZENNOTES_REMOTE_TOKEN ZENNOTES_WORKSPACE_SOURCE
export ZENNOTES_CONFIG_DIR="$root/config"
export ZENNOTES_WORKSPACE_SOURCE=terminal
export NO_COLOR=1
cd -- "$repo"
go build -o "$root/zn" ./cmd/zn
"$root/zn" init "$root/vault" --name sandbox --json
"$root/zn" create --title 'Sandbox 日本語' --tag demo --body $'A disposable note.\n\n- [ ] Try the task list\n- [ ] Open the command palette with Ctrl+G\n'

# Snippet group files. The heredocs are quoted, so the shell keeps every `$`.
mkdir -p "$root/vault/.zennotes/snippets"
cat > "$root/vault/.zennotes/snippets/markdown.md" <<'EOF'
## link

Insert a link.

```
[${1:text}](${2:url})$0
```

## table

Insert a table with two columns.

```
| ${1:Name} | ${2:Value} |
| --- | --- |
| $0 |  |
```

## callout

Insert a callout with a title.

```
> [!note] ${1:Title ${2:subtitle}}
> $0
```

## fence

Insert a code block. The longer fence holds the fence of the body.

````
```${1:go}
$0
```
````

## price

Insert a price. The `\$` is a literal dollar sign.

```
Price: \$${1:10}
```
EOF
cat > "$root/vault/.zennotes/snippets/meetings.md" <<'EOF'
## meet

Plan a meeting.

```
Meeting: ${1:name}
Date: ${date}
$0
```

## meet1

Start a note for a first meeting.

```
First meeting with ${1:name}. $0
```

## meet2

Write the name two times.

```
${1:name} says hello. Thank you, $1. $0
```
EOF
guide_body=$(cat <<'GUIDE'
This note is a test guide for the snippets of this sandbox. Do each section in order.

## 1. Start

1. Open this note in the editor.
2. With Vim on, press `i` to start insert mode.
3. Find the trigger key. With Vim on, it is `Ctrl+N`. With Vim off, it is `Tab`.

## 2. One match

1. Type `lin` on a new line.
2. Press the trigger key. The link snippet expands, and `text` is selected.
3. Read the status line. It shows `SNIPPET 1/2`.
4. Type a word. The word replaces `text`.
5. Press `Ctrl+L` or `Tab`. The cursor goes to `url`.
6. Press `Ctrl+H` or `Shift+Tab`. The cursor goes back to the first place.
7. Press `Ctrl+L` or `Tab` two times. The session ends at the final position.

## 3. The list

1. Type `mee` on a new line.
2. Press the trigger key. A list opens with `meet`, `meet1` and `meet2`. It also shows words of this note.
3. Press the trigger key again. The next item shows in the text.
4. Press `Ctrl+P` or `Shift+Tab`. The previous item shows in the text.
5. Press `Ctrl+E`. The list closes, and `mee` comes back.
6. Press the trigger key again to open the list.
7. Press `Enter`. The selected snippet expands.

## 4. Mirror

1. Type `meet2` on a new line.
2. Press the trigger key. The snippet expands.
3. Type a name. Both copies of the name change.

## 5. Nested placeholder

1. Type `callout` on a new line.
2. Press the trigger key. The snippet expands.
3. Type a letter at once. The letter replaces the whole placeholder.
4. Or press `Ctrl+L` instead. The cursor goes to `subtitle`.

## 6. Escapes and fences

1. Type `price` and press the trigger key. The text shows `Price: $10`.
2. Type `fence` on a new line and press the trigger key. A code block appears.

## 7. Smart case

1. Type `MEE` on a new line.
2. Press the trigger key. No snippet expands, because no trigger has a capital letter.
3. Type `mee` and press the trigger key. The prefix `mee` matches all three triggers.

## 8. Nested snippet

1. Expand a snippet that has a tabstop. For example, type `lin` and press the trigger key.
2. With Vim on, type `lin` again and press `Ctrl+N`. A nested session starts.
3. With Vim off, press `Tab`. The cursor goes to the next tabstop and no snippet expands.
4. To expand a snippet in a session with Vim off, see "Change the keys" in `docs/snippets.md`.

## 9. End a session

1. Press `Esc`. With Vim on, this key also leaves insert mode.
2. Press `u` with Vim on, or `Ctrl+Z` with Vim off. One undo removes the expansion.

## 10. Reload and doctor

1. Find the sandbox folder in the first line that `scripts/sandbox.sh` prints. The line starts with `Disposable workspace:`.
2. Open the file `<folder>/vault/.zennotes/snippets/meetings.md` in a second shell.
3. Add a second `## meet` snippet and save the file.
4. Run `ZENNOTES_CONFIG_DIR=<folder>/config <folder>/zn doctor` in the second shell.
5. Read the `snippets` warning. It names both places.
6. Remove the second `## meet` snippet and save the file.

The full guide is `docs/snippets.md` in the zn repository.
GUIDE
)
"$root/zn" create --title 'Snippets guide' --body "$guide_body"

if [[ "${1:-}" == "--server" ]]; then
  shift
  bind=${ZN_SANDBOX_BIND:-127.0.0.1:17878}
  "$root/zn" server install sandbox --vault "$root/vault" --bind "$bind" --no-input
  "$root/zn" server run sandbox > "$root/server.log" 2>&1 &
  server_pid=$!
  IFS= read -r ZENNOTES_REMOTE_TOKEN < "$root/config/servers/sandbox/token"
  export ZENNOTES_REMOTE_TOKEN
  export ZENNOTES_SERVER="http://$bind"
  ready=false
  for ((attempt=0; attempt<50; attempt++)); do
    if "$root/zn" list --json > /dev/null 2>&1; then ready=true; break; fi
    if ! kill -0 "$server_pid" 2>/dev/null; then break; fi
    sleep 0.2
  done
  if [[ "$ready" != true ]]; then
    printf 'Sandbox server failed to start on %s. Choose a free ZN_SANDBOX_BIND.\n' "$bind" >&2
    exit 1
  fi
fi

if [[ "${1:-}" == "--smoke" ]]; then
  "$root/zn" status --json
  doctor_out=$("$root/zn" doctor --json)
  printf '%s\n' "$doctor_out"
  if printf '%s\n' "$doctor_out" | grep -Eq '"name": *"snippets"'; then
    printf 'Sandbox smoke check failed: zn doctor reports a snippet problem in the sandbox snippet files.\n' >&2
    exit 1
  fi
  "$root/zn" config set editor.word_wrap false
  "$root/zn" read 'Sandbox 日本語.md' --json
  "$root/zn" task list --json
  printf 'Sandbox smoke checks passed.\n'
elif [[ $# -ne 0 ]]; then
  printf 'Usage: bash scripts/sandbox.sh [--server] [--smoke]\n' >&2
  exit 2
else
  printf 'Disposable workspace: %s\nExit the TUI with :qa. Everything is removed on exit.\n' "$root"
  unset NO_COLOR
  "$root/zn" tui
fi
