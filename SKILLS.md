# bd-lite Skills Reference

## Quick Reference

```bash
# Find work
bd ready                           # Unblocked issues ready to work
bd list                            # Open issues (excludes closed)
bd list --status=open              # Filter by status
bd list --status=in_progress       # Your active work
bd list --all                      # Include closed issues
bd show <id>                       # Full issue details with comments

# Create issues
bd create "title" -t bug -p 1 --json
bd create "title" --deps blocks:<id>
# Priority: 0-4 (0=critical, 2=medium, 4=backlog). NOT "high"/"medium"/"low"

# Update
bd update <id> --status in_progress
bd update <id> --priority 1 --assignee=username
bd close <id> --reason "Done"
bd close <id1> <id2> ...           # Close multiple at once

# Dependencies
bd dep add <blocked> <blocker>     # "blocked depends on blocker"
bd dep remove <blocked> <blocker>
bd dep tree <id>                   # Visualize dependency tree

# Comments
bd comment <id> "text"

# Maintenance
bd cleanup                         # Archive and delete closed issues
bd cleanup --older-than 30         # Only issues closed 30+ days ago
bd cleanup --no-archive            # Delete without archiving
bd cleanup --dry-run               # Preview what would happen
```

All commands support `--json` for machine-readable output.

## Issue Types & Priorities

| Type | Use for |
|------|---------|
| `bug` | Something broken |
| `feature` | New functionality |
| `task` | Work items (tests, docs, refactoring) |
| `epic` | Large features with subtasks |
| `chore` | Maintenance (dependencies, tooling) |

| Priority | Meaning |
|----------|---------|
| `0` | Critical (security, data loss, broken builds) |
| `1` | High (major features, important bugs) |
| `2` | Medium (default) |
| `3` | Low (polish, optimization) |
| `4` | Backlog (future ideas) |

## Workflow

1. `bd ready` - find unblocked work
2. `bd update <id> --status in_progress` - claim it
3. Work on it
4. Discover new issues? `bd create "..." --deps blocks:<id>`
5. `bd close <id> --reason "Done"` - mark complete
6. Commit `.beads/issues.jsonl` with code changes

## Rules

- Use `--json` for programmatic use
- File issues immediately when discovered mid-task
- Make titles self-contained with file/method names
- Commit `.beads/issues.jsonl` with related code changes

## What's NOT in bd-lite

These are full beads features. If you need them, use [beads](https://github.com/steveyegge/beads):

- `bd sync`, `bd status`, `bd blocked`, `bd search`, `bd doctor`
- `bd human`, `bd prime`
- `bd gate` (workflow coordination)
- `bd cook` / `bd formula` (workflow templates)
- `--suggest-next` on close
- `--after` date filtering on list
- `discovered-from` dependency type
