# Changelog

bd-lite had no tagged releases before 0.1.0, so builds before it carry a Go
pseudo-version. To see when an installed binary was built, run
`go version -m $(which bd)` and read `vcs.time`. The dates below are when each
change landed on `main`, so anything dated after your `vcs.time` is missing
from your binary.

## [0.1.0] - 2026-11-16

First tagged release. It covers everything since the initial commit on
2026-02-26.

### Upgrading a shared tracker

These matter when more than one person, or more than one bd-lite build, writes
the same `issues.jsonl`.

- **Upgrade everyone together.** Builds from before 2026-09-01 sort the file by
  `created_at`; this release sorts it by `created_by`, then `id`. Every write
  rewrites the whole file, so an old build and a new one alternating on one
  tracker reorder it back and forth, and every such commit conflicts with the
  last.
- **The first write after upgrading reorders the file.** Expect one large diff
  in `issues.jsonl` that changes no issue content.
- **Builds from before 2026-07-09 drop fields they don't model**, including
  `created_by`, `closed_by`, and upstream beads' `design` and `notes`, on any
  write. Builds from 2026-07-09 onward keep them.

### Added

- When GitHub has a newer release, every command prints a notice to stderr
  with the command that downloads it beside the running binary and moves it
  into place. The check runs at most once a day, is cached in
  `~/.cache/bd-lite/latest.json`, stays quiet when it fails, and is skipped for
  untagged or modified builds and under `BD_NO_UPDATE_CHECK=1`. bd never updates
  itself. Builds older than this one have no notice and need one manual update.
  (2026-11-13)
- Releases carry a prebuilt `bd-linux-amd64`, published by `make release`.
  (2026-11-13)
- `bd --version` (and `-v`) prints the version stamped from the git tag at
  build time. Untagged builds print a pseudo-version, and a build from a
  modified tree ends in `+dirty`. (2026-11-13)
- Under Claude Code (`CLAUDECODE=1`) with no `$BD_ACTOR`, new issues and
  comments are attributed to `<human>/claude`, the slash convention bd-lite-tui
  already parses. (2026-11-06)
- `bd close` and `bd update --status closed` record `closed_by`; reopening
  clears it. `bd show` appends `by <closer>` to the Closed line. (2026-11-06)
- `bd cleanup` writes a full backup of `issues.jsonl` to
  `.beads/.cleanup-backups/` before deleting anything, including under
  `--no-archive`. It refuses to run non-interactively without `--yes` and asks
  for y/N confirmation on a terminal. (2026-07-13)
- `make fmt-check` fails when any of bd-lite's own Go files are not gofmt-clean.
  The vendored `beads/` tree is excluded. (2026-07-09)
- New issues and comments record `created_by`, resolved from `$BD_ACTOR`, then
  `git config user.name`, then `$USER`, then the OS account name. The field is
  upstream beads' wire format. Existing issues are not backfilled. (2026-07-09)
- `bd list` and `bd ready` show each issue's age (`5d`, `3mo`, `2y`). This
  changes their default output for anything parsing it. (2026-07-09)
- IDs resolve from a bare suffix or a prefix of it (`prv` or `pr` for
  `bd-lite-prv`) when the match is unique, in every command that takes an ID.
  (2026-06-30)
- `bd <cmd>` runs a `bd-<cmd>` executable from `PATH` when `<cmd>` is not
  built in. (2026-02-26)
- `install.sh` builds and installs `bd` to `$INSTALL_DIR` (default
  `~/.local/bin`). (2026-02-26)
- Aliases and short flags matching beads: `create` (was `add`) with alias
  `new`, `show` alias `view`, `init -p`, `update -s/-p/-d/-t`, `close -r`.
  (2026-02-26)
- A legacy `beads.jsonl` is renamed to `issues.jsonl` on first load.
  (2026-02-26)

### Changed

- `issues.jsonl` is sorted by `created_by`, then `id`, instead of by
  `created_at`. Concurrent `bd create` by two people no longer always writes the
  same line at the end of the file, so merges of those commits mostly stop
  conflicting. Issues with the same `created_at` no longer reshuffle on every
  write. (2026-09-01)
- `bd comment` attributes comments through the same chain as `created_by`
  instead of the OS username, so one person no longer appears under two
  spellings in one file. Existing comments keep their stored author.
  (2026-07-09)
- `bd cleanup` absorbs the former `archive` command. It archives to
  `archive.jsonl` and deletes by default; `--no-archive` deletes only.
  (2026-02-26)

### Fixed

- A tracker whose prefix contains a hyphen (`hamilos-metadata-*`) and that has
  no `.beads/config.yaml` no longer fails every command with "issues have mixed
  prefixes". (2026-11-13)
- JSON keys bd-lite does not model survive the load and save round trip on
  issues, dependencies and comments. Previously any write dropped them from
  every issue in the file, including upstream beads' `design`, `notes` and
  `Dependency.metadata`. (2026-07-09)
- `bd list` pads the ID column to the widest ID, so the priority, type and age
  columns line up. (2026-07-09)
- `bd dep tree` follows an issue's own dependencies, matching `bd show` and
  `bd ready`, and nests children instead of printing them flat. Cycles print
  once. (2026-07-06)
- `bd show` prints dependencies as `depends on <id> (<type>)`; it previously
  read as the reverse of what was stored. (2026-05-27)
- `bd comment --json` returns the new comment, not the whole issue, as beads
  does. (2026-02-26)
