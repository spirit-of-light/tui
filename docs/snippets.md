# Snippets

A snippet is a short word that expands into a block of text. The text can
have places that you fill in, one after the other. zn reads the snippets of
a local vault from files in the vault.

## Quick start

1. Make the file `.zennotes/snippets/markdown.md` in your vault.
2. Write the example from "The group file" below in the file.
3. Open a note in the editor. In Vim mode, press `i`.
4. Type `link` and press the trigger key.
5. The text `[text](url)` appears, and `text` is selected.
6. Type the link text.
7. Press the key for the next tabstop.

zn reloads the snippets when a file changes. It is not necessary to restart zn.

## The trigger key

The trigger key expands the snippet that matches the word before the cursor.

| Vim mode | Trigger key |
| --- | --- |
| On | `Ctrl+N` |
| Off | `Tab` |

The trigger key acts only when all of these are true:

- The note is open in the editor, and it has finished loading.
- With Vim on, the editor is in insert mode. The command line and replace mode do not count.
- The cursor is directly after a word. A word has letters, digits and underscores.

In all other cases, the key has its usual function. These are some examples:

- With no snippet that matches the word, `Tab` indents and `Ctrl+N` starts Vim keyword completion.
- With Vim off, `Ctrl+N` makes a new note and `Ctrl+P` opens the note search, when no snippet list is open.
- With a trigger that matches, `Tab` expands the snippet and does not indent a list item.

A paste never expands a snippet. A running Vim keyword completion keeps its `Ctrl+N` and `Ctrl+P` keys.

To use other keys, set `snippet.expand` in `[terminal.keymaps]` of `config.toml`.
An override works in both Vim modes. See "Change the keys".

## The smart-case match

The word before the cursor is the prefix. zn matches the prefix with the start of each trigger.

- A prefix with no capital letter matches in any case. The prefix `sn` matches `snip` and `Snip`.
- A prefix with a capital letter matches only the same case. The prefix `Sn` matches `Snip` but not `snip`.

The prefix can be the start of a trigger. You do not need to type the whole trigger.

## One match or more

- One trigger matches: the snippet expands at once. The prefix goes away.
- Two or more triggers match: a list opens under the prefix. The list is described below.
- No trigger matches: the key has its usual function.

## The snippet list

The list shows each matching trigger in the order of the files. Then it shows the words of the
current note that match the prefix. Each word shows one time. A word that is also a trigger
shows only as a trigger. A word of the note never opens the list by itself.

The first item shows in the text at once. Each row of a snippet shows its trigger, its
description and its group. A row of a word shows the word and `word`. The list shows at most
ten rows and scrolls.

| Key | Function |
| --- | --- |
| `Ctrl+N`, `Tab` and the trigger key | Select the next item. After the last item, select the first item. |
| `Ctrl+P`, `Shift+Tab` | Select the previous item. Before the first item, select the last item. |
| `Enter`, `Ctrl+Y` | Use the selected item. A snippet expands. A word stays as text. |
| `Ctrl+E` | Close the list. The text that you typed comes back. |
| `Esc` | Close the list. The selected text stays. With Vim on, insert mode ends. |
| Any other key | Close the list. The selected text stays. The key then does its usual function. |

The note shows the trigger word and not the body, until you use the item.

One undo gives back the prefix that you typed. Press `Ctrl+Z` with Vim off, or `u` with Vim on.

## The snippet body

A body is text with these parts:

| Part | Meaning |
| --- | --- |
| `$1`, `${1}` | A tabstop. The cursor goes to the tabstops in the order of their numbers. |
| `${1:text}` | A tabstop with a placeholder. The placeholder can contain other tabstops. |
| `$1` two or more times | Mirrors. All places with the same number show the same text. |
| `$0` | The final position. With no `$0`, the final position is the end of the text. |
| `${title}` | The title of the note. |
| `${date}` | The date, as `2026-11-22`. |
| `${date:FORMAT}` | The date in a format. It uses the same format codes as the date token of a note template. |
| `${time}` | The time, as `18:25`. |
| `${week}` | The number of the ISO week, with two digits. |
| `\$` and `\}` | A literal `$` and a literal `}`. |

A line break in the body starts a new line with the same indent as the line of the cursor.
A tab character in the body becomes one indent level.

A `$` that does not start a valid part stays as text. A variable that has no value stays as
its source text. Write `\$` for each literal dollar sign, for example in a price.

## The session keys

A snippet that has a tabstop starts a session. The status line shows `SNIPPET 1/3`.

| Action | Default keys |
| --- | --- |
| `snippet.nextTabstop` | `Ctrl+L`, `Tab` |
| `snippet.prevTabstop` | `Ctrl+H`, `Shift+Tab` |

The keys act only while a session runs. Some terminals send `Ctrl+H` for Backspace. Use
`Shift+Tab` in these terminals. The session ends at the final position. But if `$0` occurs two or more times, the session stays.
Then an edit goes to all copies, until you press `Esc` or the key for the next tabstop. With Vim off, `Esc`
also ends the session.

A snippet that you expand inside a session starts a nested session. The older session is
active again when the nested session ends.

While a session runs, `Tab` goes to the next tabstop. The trigger key does not expand a
snippet with Vim off, because `Tab` has the session function first. With Vim on, `Ctrl+N`
expands a snippet also in a session.
To expand a snippet in a session with Vim off, set `snippet.expand` to another key, for example
`"snippet.expand" = "Alt+S"`. See [Change the keys](#change-the-keys).

## Change the keys

Set each action in the `[terminal.keymaps]` table of `config.toml`:

```toml
[terminal.keymaps]
"snippet.expand" = "Alt+S"
"snippet.nextTabstop" = "Ctrl+J"
```

- A value replaces all default keys of the action.
- An empty value `""` removes the keys of the action. With `snippet.expand`, no key expands a snippet.
- Only a single key acts. A key sequence is stored but does nothing.
- The commands `:bind snippet.expand Alt+S` and `:unbind snippet.expand` write the same table and act at once.
- The command `:keymaps` lists the keys of each action.

When you set `snippet.expand`, the list keeps `Ctrl+N`, `Tab`, `Ctrl+P` and `Shift+Tab`.
The new key also selects the next item.

## The group file

zn reads each file `.zennotes/snippets/<group>.md`. A group file has these rules:

- The file is directly in `.zennotes/snippets/`. A file in a subdirectory is not read.
- The file name ends with `.md`. A hidden file is not read.
- The group name is the file name without `.md`. The name can have spaces.
- A frontmatter block at the start of the file is skipped.
- A heading of level 2 (`## trigger`) starts a snippet. The heading text is the trigger.
- The trigger is one word. It has letters, digits and underscores only.
- The first paragraph after the heading, before the first fence, is the description. Other paragraphs are notes for you.
- The first fenced code block after the heading is the body. Other fenced blocks are ignored.
- The info string of a fence is ignored.
- Text that is not in a snippet, and headings of other levels, are plain text.

A fence uses at least three backticks or three tildes. The closing fence uses the same
character and at least the same length. To put a fence in a body, use a longer fence around it.

This is one full example:

`````markdown
---
title: Markdown snippets
---

Text before the first heading is ignored.

## link

Insert a link.

```
[${1:text}](${2:url})$0
```

## callout

Insert a callout with a title.

```
> [!note] ${1:Title ${2:subtitle}}
> $0
```

## price

Insert a price. The `\$` is a literal dollar sign.

```
Price: \$${1:10}
```

## today

Insert the date and the time.

```
${date} ${time}
```

## fence

Insert a code block. The longer fence holds the fence of the body.

````
```${1:go}
$0
```
````
`````

## Order and duplicates

zn reads the files in byte order of their names. A capital letter comes before a small
letter, so `B.md` comes before `a.md`. In one file, the snippets keep the order of the headings.

Two snippets can have the same trigger, as text. The first snippet wins. zn ignores the later
snippet. A trigger is case-sensitive, so `meet` and `Meet` are two triggers.

## Problems in a group file

zn shows no error in the editor for a bad snippet. Use `zn doctor`. A problem is a warning and
does not change the exit status. `zn doctor` reports these problems:

- A heading that is not one word. zn ignores the snippet.
- A heading with no fenced block. zn ignores the snippet.
- A fenced block that does not close. zn ignores the snippet.
- A file that zn cannot read. zn ignores the file.
- A duplicate trigger. The line names the file and the line that zn uses and that zn ignores.

A vault with no problem has no `snippets` line in the output. With `--json`, a duplicate has
the trigger and both places in the `snippet` object of the check.

## A remote vault

The snippets of a vault are files on your computer. A vault on a server has no snippets in
this version. The first time in a zn run that you press the trigger key after a word, zn shows
`Snippets are not available for a remote vault.` The key then has its usual function. `zn
doctor` skips the snippet check for a remote vault.
