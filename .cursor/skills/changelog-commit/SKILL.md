---
name: changelog-commit
description: >-
  Generates a complete, descriptive Conventional Commit message in changelog
  format from uncommitted git changes. Use when the user asks for a commit
  message, conventional commit, changelog commit, or is committing work that
  will appear in the GoReleaser/GitHub release notes.
---

# Changelog Conventional Commit

Write the commit message as a **release changelog entry**. GoReleaser
(`changelog.use: github` in `.goreleaser.yaml`) publishes each commit **subject**
as a line in the GitHub release notes. Vague subjects become useless changelog
rows.

Do **not** commit unless the user explicitly asks. Generate the message first.

## Inspect the working tree

Run these in parallel before drafting:

```bash
git status --short
git diff HEAD
git diff --cached
git log -12 --pretty=format:'%s'
```

Read untracked files that will be part of the change. Infer user-facing impact
from the diff, not from filenames alone.

If there is nothing to commit, say so and stop.

## Message format

```
<type>(<scope>): <complete changelog subject>

- User-facing change or reason
- Another user-facing change or reason
```

Use `!` after the type/scope (`feat(cli)!: ...`) when the change is breaking,
and add a `BREAKING CHANGE:` footer.

### Subject (this is the published changelog line)

- One line, imperative mood, no trailing period
- Standalone: a user reading GitHub Releases must understand the change
  without opening the diff
- Complete over terse. Prefer ~72 characters, but do not drop meaning to
  hit a length cap
- Describe outcome and audience impact, not the files touched
- Include the most important user-visible behavior in the subject; the
  pipeline does not publish the body

### Body (git history; not the GitHub changelog)

- Changelog-style bullets (`- `), not a file list
- Cover every user-visible or operator-visible change in the working tree
- Explain **why** when the subject cannot
- Mention breaking behavior, migration, or default changes explicitly
- Omit trivia: formatting-only, comment-only, or test-only details unless
  that is the entire commit

## Types

| Type | Use for | In GitHub changelog? |
|------|---------|----------------------|
| `feat` | New user-facing capability | Yes |
| `fix` | Bug fix or incorrect behavior | Yes |
| `perf` | User-noticeable speed/resource win | Yes |
| `refactor` | Internal change with no behavior change | Yes |
| `chore` | Maintenance that still matters to operators (deps, packaging) | Yes |
| `build` | Build/release packaging that affects artifacts | Yes |
| `docs` | Documentation only | No (filtered) |
| `test` | Tests only | No (filtered) |
| `ci` | CI workflow only | No (filtered) |

Pick the **primary** type from the user-facing impact. A feature plus tests is
`feat`, not `test`. A bug fix plus docs is `fix`, not `docs`.

If the only changes would be filtered (`docs` / `test` / `ci`), still use the
correct type and tell the user this commit will **not** appear in the release
changelog.

## Scopes

Use one scope when the change is concentrated. Omit the scope when it spans
several areas equally.

| Scope | Area |
|-------|------|
| `downloader` | Fetch, convert, tag, progress, metadata |
| `musicbrainz` | Lookup, matching, cover art sources |
| `tools` | ffmpeg, yt-dlp, Deno/Node detection and install |
| `cli` | Commands, flags, help, version |
| `config` | Config files and defaults |
| `installer` | `scripts/install.sh`, `scripts/install.ps1`, install notes |
| `release` | GoReleaser, archives, checksums, release workflow |

## Split vs one commit

Recommend **separate commits** (and draft a message for each) when the working
tree mixes unrelated user-facing changes — for example a downloader feature and
an installer bugfix. Each subject becomes its own changelog line.

Keep **one commit** when the changes are one story (implementation + tests +
docs for the same behavior).

## Quality bar

**Do**

- `feat(downloader): show sectioned progress while fetching and tagging audio`
- `fix(installer): install ffmpeg and yt-dlp as standalone binaries on macOS`
- `feat(cli)!: rename the binary and commands from smart-fetcher to iTurtle`

**Do not**

- `feat: iturtle-smart-fetcher`
- `fix: .gitignore`
- `docs: update README.md`
- `chore: improvements`
- Subjects that only name a file or package

Existing history in this repo is often too terse. Do not imitate it.

## Output

1. State the primary type, scope, and whether it will appear in the release
   changelog
2. If a split is better, say so and provide one message per commit
3. Print the exact commit message in a fenced `text` block, ready to use
4. Commit only when the user asked — then use this message as-is via HEREDOC
   and follow the repo's git safety rules
