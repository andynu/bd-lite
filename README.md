# bd-lite

This project owes everything to [Steve Yegge](https://github.com/steveyegge) and the original [beads](https://github.com/steveyegge/beads) issue tracker. Beads is a seriously cool project doing all kinds of wild, ambitious stuff -- distributed issue tracking with SQLite, Dolt, a daemon, RPC, compaction, multi-repo sync, and more. It's the real deal for AI agent workflows.

But I didn't need all of that. I just needed the six commands I actually use every day, a single JSONL file as the source of truth, and nothing else. So bd-lite strips beads back down to basics: same data format, no database, no daemon, no sync. Just issues in a file.

## Install

```bash
go install bd-lite@latest
```

Or build from source:

```bash
git clone <this-repo>
cd bd-lite
go build -o bd .
```

## Quick Start

```bash
bd init --prefix myproject
bd create "Fix login bug" -t bug -p 1
bd create "Add dark mode" -t feature
bd list
bd update myproject-abc --status in_progress
bd ready
bd close myproject-abc --reason "Fixed"
```

## Commands

| Command | Description |
|---------|-------------|
| `bd init [--prefix X]` | Create `.beads/` directory with config |
| `bd create "title" [-p -t -d -a -l --deps]` | Create an issue |
| `bd update <id> [--status --priority ...]` | Modify issue fields |
| `bd close <id> [--reason "..."]` | Close an issue |
| `bd show <id> [<id>...]` | Display full issue details |
| `bd list [--status --priority --type --all]` | List issues (excludes closed by default) |
| `bd ready` | Show unblocked work |
| `bd dep add <id> <depends-on>` | Add a blocking dependency |
| `bd dep remove <id> <depends-on>` | Remove a dependency |
| `bd dep tree <id>` | ASCII dependency tree |
| `bd cleanup [--older-than N --dry-run --no-archive]` | Archive and delete closed issues |
| `bd comment <id> "text"` | Add a comment |

All commands support `--json` for machine-readable output.

## Data Model

Everything lives in `.beads/issues.jsonl` -- one JSON object per line, wire-compatible with the full beads format. Closed issues can be archived to `.beads/archive.jsonl` and removed with `bd cleanup`.

**Issue fields:** id, title, description, status, priority (0-4), issue_type, assignee, labels, dependencies, comments, timestamps.

**Statuses:** open, in_progress, blocked, closed.

**Types:** bug, feature, task, epic, chore.

**Priorities:** 0 (critical) through 4 (backlog). Default is 2.

## ID Generation

IDs use the same scheme as beads: SHA256 hash of content, encoded as base36, with adaptive length (3-8 characters) based on the birthday paradox to keep collision probability under 25%. Partial ID matching works on all commands.

## What's Not Here

If you need any of these, use [beads](https://github.com/steveyegge/beads):

- SQLite storage
- Dolt versioning
- Background daemon / RPC
- Git hook integration
- Compaction
- Multi-repo sync
- Custom statuses
- Hierarchical (child) issues
- Soft-delete / tombstones

## License

MIT
