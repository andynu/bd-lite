---
name: verify
description: Build and drive bd-lite end-to-end in an isolated scratch workspace
---

# Verifying bd-lite changes

bd-lite is a single-module Go CLI (the vendored `beads/` dir is a separate
module and is excluded from `go build ./...` / `go test ./...`).

## Build + isolated workspace

```bash
go build -o "$SCRATCH/bd-test" .
mkdir -p "$SCRATCH/play" && cd "$SCRATCH/play"
"$SCRATCH/bd-test" init          # creates ./.beads with prefix from dir name
```

`bd` finds `.beads/` by walking up from cwd, so running inside the scratch
dir fully isolates state from the repo's own `.beads/`.

## Flows worth driving

```bash
bd create "title" -t epic --json     # parse .id from JSON
bd dep add <epic> <child>            # epic depends on child
bd show <epic>                       # deps render "depends on <id> (<type>)"
bd dep tree <epic>                   # nested tree of what epic depends on
bd ready                             # epic absent while children open
bd close <children...> -r done && bd ready   # epic appears
```

Gotchas:
- Direction: `dep add A B` = "A depends on B". `dep tree A` follows A's
  dependencies downward; a leaf prints as a single line.
- Cycles must terminate: revisited node prints once, not re-expanded.
- `--json` works on every command; `dep tree --json` emits a flattened
  list with `depth` fields.
