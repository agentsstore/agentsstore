# agentsstore

A small Go HTTP service that aggregates multiple Claude Code plugin
marketplaces (Git repos and HTTP endpoints) into a single URL. Team
members configure their Claude Code once, pointing at the aggregator;
adding/removing upstream sources is done through the admin web UI or
the JSON admin API.

## Quick start

```bash
go build -o agentsstore .
./agentsstore                  # listens on :8080
```

Open http://localhost:8080/admin/ in your browser.

## Configure Claude Code to use the aggregator

In your `~/.claude/settings.json`:

```json
{
  "marketplaces": {
    "team-aggregated": {
      "url": "http://aggregator.local:8080/marketplace.json"
    }
  }
}
```

Claude Code reads `/marketplace.json` and then fetches each plugin via
`/plugins/{source}/{plugin_path}`.

## Configuration

The service reads `config.yaml` in the working directory (or the path in
`AGENTSSTORE_CONFIG`). Example:

```yaml
server:
  listen: ":8080"
  data_dir: "./data"
  base_url: ""           # optional; auto-derived if empty

sources:
  - name: "anthropic-official"
    type: "git"
    url: "https://github.com/anthropics/claude-code.git"
    ref: "main"
    enabled: true
```

## Source types

- `git`: shallow-clones the repository on refresh. Reads
  `.claude-plugin/marketplace.json` from the cloned tree (the
  Claude Code convention). Use `ref` to pin a branch/tag.
- `http`: takes a **base directory URL** and fetches
  `<base>/.claude-plugin/marketplace.json` plus each plugin's
  files referenced by the `source` field. The base URL is the
  repository/directory root, not the manifest file itself.

## Source format requirement

Each source must be a Claude Code marketplace: a repository or
directory containing `.claude-plugin/marketplace.json` at its root.
That manifest lists the available plugins. A repo that is itself a
plugin (e.g. has only `.claude-plugin/plugin.json`) is **not** a
marketplace and cannot be aggregated — see
[`obra/superpowers`](https://github.com/obra/superpowers) for an
example of a non-marketplace repo.

## Admin API

See the design doc for the full surface:
`docs/superpowers/specs/2026-06-11-claude-code-marketplace-aggregator-design.md`.

## Development

```bash
go test ./...
go vet ./...
```

No Docker, no system git required.
