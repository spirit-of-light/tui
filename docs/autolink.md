# Autolink

Autolink adds a `[[link]]` at the first reference to each other note. It works
from the command line with `zn autolink` and in the terminal app with
`:autolink`. Before the command writes, you review each change. This page gives
the rules, the review and the settings, and it describes the `aliases` list
that autolink reads.

## Run the command

### From the command line

`zn autolink <path>` adds links to one note. `zn autolink --all` adds links to
each live note in the vault.

```bash
zn autolink "Project plan.md"
zn autolink --all
```

The review needs a terminal. Without a terminal, the command stops with an
error and changes no note.

Caution: Make a copy of the vault before you run `zn autolink --all`. The
command keeps no backup, and you cannot undo its changes.

### In the terminal app

`:autolink` or the palette entry "Autolink" adds links to the open note. The
command plans from the text in the buffer, not from the file on disk. It does
not save the note. One `u` in normal mode reverts all the changes of the
buffer.

`:autolink --all` adds links to each live note in the vault. If an open note
has unsaved changes, the command names it and stops. Save these notes first.
After the review, the command saves each open note that it changed. It writes
the other notes to disk.

### On a server vault

The two commands work on a local vault only. On a server vault, they stop with
an error before they read or write a note.

## What the command links

The command links the first reference to each other note. A reference is the
title of a note, or one of its aliases, as a whole word. The case does not
matter.

The last word of a title also matches its English word forms:

- singular and plural: "notes" matches the note "Note"
- `-y` and `-ies`: "theories" matches "Theory"
- `-es`: "boxes" matches "Box"
- `-ic` and `-ics`: "thermodynamic" matches "Thermodynamics"
- irregular plurals from a fixed list: "matrices" matches "Matrix"

The new link keeps the text that you wrote. `theories` becomes
`[[Theory|theories]]`, and `Theory` becomes `[[Theory]]`.

The command never links a note to itself. It never links to a note in the
trash, in `archive/`, in the template folder or in the Typst preamble folder.
It does not change a note in these places.

## What the command does not change

The command does not add, move or remove a link in these parts of a note:

- the frontmatter
- fenced code and inline code
- math, with `$...$`, `$$...$$` and `\begin...\end`
- headings
- URLs and `<...>` links
- HTML elements
- tables
- comments, with `<!-- ... -->` and `%% ... %%`
- task lines
- the title line of a callout
- the full text of a link, an embed or a Markdown link

A `$` that is a currency sign does not start math. "It costs $5 and $10" stays
prose.

## Links that are already in a note

A note keeps one link to each other note, at the first reference. If a link to
a note comes after the first reference, the command moves the link to the
first reference. The old position keeps the text that you saw.

The setting `autolink_remove_duplicates` in `[terminal]` controls the other
links to the same note. With `true` (the default), each other link becomes
plain text. With `false`, the other links stay.

```toml
[terminal]
autolink_remove_duplicates = false
```

These links do not count as the link to a note, and the command never moves
or removes them:

- an embed, such as `![[X]]`
- a link with a heading anchor, such as `[[X#Part]]`
- a link in one of the parts that the command does not change, for example a
  task line, a table or a heading

A Markdown link such as `[text](X.md)` counts as a link to `X`, the same as a
wikilink.

## Conflicts

When references to two notes overlap, the command asks which note to use. An
example is "Linear Algebra", which contains "Algebra". The menu shows "skip"
and each note. After your choice, a later reference to the other note can
still get its link.

Two notes with the same title or alias also give a conflict. If you select one
of them, the link uses its path, for example `[[b/X|X]]`.

## The review

Each change shows its type (add, move or remove), the note path, the line
number, the text that changes and the line of text. A move also shows the old
link and its line.

```text
  1  move   Physics.md:2  thermodynamic → [[Thermodynamics|thermodynamic]]
       Context: The thermodynamic process uses linear algebra.
       Old link: Physics.md:4  [[Thermodynamics|thermo]] → thermo
```

On the command line, type the numbers of the changes to omit. An empty line
keeps all changes. At the last question, `y` writes the notes, and any other
answer cancels.

In the terminal app, Tab omits or keeps a change, Enter applies the selected
changes, and Esc cancels.

## Aliases

A note can list other names for itself in its frontmatter:

```yaml
---
aliases: [LA, Lin Alg]
---
```

A block list (`- LA`) is also correct. A link such as `[[LA]]` resolves to the
note with the alias `LA` when no path and no title match. The link is also a
backlink of that note. A rename of the note does not change a link that
resolves through an alias.

The desktop app does not read `aliases` for links. It shows `[[LA]]` as a link
to a note that does not exist.
