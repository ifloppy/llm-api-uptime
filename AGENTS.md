# Contributor Instructions

## CodeGraph

- Install CodeGraph and run `codegraph init` once from the repository root to create the local index.
- Use `codegraph explore "<question or symbols>"` before broad text searches when learning architecture, tracing a flow, debugging, or locating code to change.
- Run `codegraph status` to verify index health and check for pending files when results appear incomplete or stale.
- The watcher normally keeps the graph current. Run `codegraph sync` after branch switches, bulk changes, or whenever the watcher is unavailable.
- Before changing a shared symbol, run `codegraph impact <symbol>` to inspect its blast radius.
- Use `codegraph affected <changed-files...>` or `git diff --name-only | codegraph affected --stdin` to select relevant tests after a change.
- The `.codegraph/` directory is a local generated index. Never add or commit it.

## Development

- Keep changes focused and preserve user data. In particular, cleanup commands must not remove `data/` or database files.
- Run `go vet ./...` and `go test ./...` before submitting Go changes.
