# 🧠 Roam CLI — Your graph in the terminal.

Roam Research in your terminal. Query, create, update, and automate pages, blocks, daily notes, and more.

## Features

- **Authentication** - cloud tokens and encrypted graph support
- **Pages** - create, update, list, and delete pages
- **Blocks** - create, update, move, delete, and reorder blocks
- **Daily notes** - quick capture and context retrieval
- **Search** - full-text and tag/status searches
- **Query** - Datalog `query`, `pull`, and `pull-many`
- **Batch & import** - run batch actions, import markdown
- **Append API** - append-only captures (works with encrypted graphs)
- **Local API** - undo/redo, file ops, shortcuts, user upsert
- **Structured output** - text, json, ndjson, yaml, table (jq filtering)

## Installation

### Homebrew

```bash
brew install salmonumbrella/tap/roam
```

## Quick Start

### 1. Authenticate

Choose one of two methods:

**Cloud graph:**
```bash
roam auth login
```

**Encrypted graph (desktop app required):**
```bash
roam auth login --encrypted-graph --graph devgraphencrypted
```

### 2. Test Authentication

```bash
roam auth status
roam page list --limit 5
```

## Configuration

### Graph Selection

Specify the graph using either a flag or environment variable:

```bash
# Via flag
roam page list --graph my-graph

# Via environment
export ROAM_GRAPH_NAME=my-graph
roam page list
```

### Environment Variables

- `ROAM_GRAPH_NAME` - Default graph name
- `ROAM_API_TOKEN` - API token for cloud graphs
- `ROAM_KEYRING_BACKEND` - Keyring backend: auto, keychain, file
- `ROAM_KEYRING_PASSWORD` - Password for file-based keyring
- `ROAM_OUTPUT` - Output format: `text` (default), `json`, `yaml`, `table`

## Security

### Credential Storage

Credentials are stored securely in your system's keychain:
- **macOS**: Keychain Access
- **Linux**: Secret Service (GNOME Keyring, KWallet)
- **Windows**: Credential Manager

## Rate Limiting

The Roam API enforces rate limits. The CLI automatically handles rate limiting with:

- **Exponential backoff** - Retries with increasing delays
- **Retry on 429** - Retries when rate-limited
- **Maximum retry attempts** - Stops after a small number of retries

## Commands

### Authentication

```bash
roam auth login            # Store credentials in keychain
roam auth logout           # Clear stored credentials
roam auth status           # Show authentication status
```

### Pages

```bash
roam page get <title>               # Get page content
roam page get <title> --render markdown
roam page create <title>            # Create new page
roam page create <title> --uid <u>  # Create with custom UID
roam page update <uid> --title "New Title"
roam page update <uid> --children-view numbered
roam page delete <uid>
roam page list --limit 20
```

### Blocks

```bash
roam block get <uid>
roam block create --parent <uid> --content "text"
roam block create --page-title "My Page" --content "text"
roam block create --daily-note 01-11-2026 --content "text"
roam block update <uid> --content "new text"
roam block update <uid> --heading 2
roam block update <uid> --props '{"key":"value"}'
roam block move <uid> --parent <uid>
roam block move <uid> --page-title "My Page"
roam block move <uid> --daily-note 01-11-2026
roam block delete <uid>
```

### Daily Notes

```bash
roam daily get
roam daily get --date 2026-01-10
roam daily add "quick capture"
roam daily context --days 7
roam remember "quick note"
```

### Search

```bash
roam search "project"
roam search tags "meeting"
roam search status TODO
roam search refs <uid>
```

### Query

```bash
roam query '[:find ?t :where [?e :node/title ?t]]'
roam pull '[:node/title "January 10th, 2026"]'
roam pull-many '[:node/title "A"]' '[:node/title "B"]'
```

### Batch & Import

```bash
roam batch --file actions.json
roam batch --file actions.json --native
roam import notes.md --page "Imported Notes"
```

### Append (Encrypted Graphs)

```bash
roam append --page "My Page" --content "New block"
roam append --daily-note --content "Quick thought"
roam append --block-uid abc123 --content "Child block"
roam append --page "Notes" --nest-under "Captures" --content "Quick capture"
roam append --file payload.json
```

### Local API (Desktop App)

```bash
roam local undo
roam local redo
roam local reorder <parent-uid> <uids>...
roam local upload image.png
roam local download <url> [output]
roam local delete <url>
roam local shortcut add <page-uid> [idx]
roam local shortcut remove <page-uid>
roam local user upsert <user-uid> --display-name "Name"
roam local call <action> --args '[...]'
```

## Output Formats

### Text

```bash
roam page get "January 10th, 2026"
```

### JSON

```bash
roam page get "January 10th, 2026" -o json
```

### NDJSON

```bash
roam page list -o ndjson
```

### jq Filtering

```bash
roam page list -o json --query '.[] | {title: .title, uid: .uid}'
```

## Error Output

Structured output emits structured errors on stderr (JSON/YAML) so stdout remains clean for piping.

```bash
roam page get "Missing Page" -o json --error-format json
```

## Examples

### Create a daily note entry

```bash
roam daily add "Standup notes"
```

### Run a Datalog query

```bash
roam query '[:find ?uid ?str :where [?b :block/uid ?uid] [?b :block/string ?str]]'
```

### Append nested blocks

```bash
echo -e "Parent\n  Child" | roam append --page "Notes"
```

### Move a block to a daily note

```bash
roam block move <uid> --daily-note 01-11-2026
```

### Batch operations

```bash
roam batch --file actions.json --native
```

## Global Flags

| Flag | Description |
|------|-------------|
| `-g, --graph` | Graph name (env: ROAM_GRAPH_NAME) |
| `--token` | API token (env: ROAM_API_TOKEN) |
| `-o, --output` | Output format: text, json, ndjson, yaml, table |
| `--format` | Alias for `--output` |
| `--query` | jq expression to filter JSON output |
| `--query-file` | Read jq expression from file (use `-` for stdin) |
| `--error-format` | Error output format: auto, text, json, yaml |
| `--quiet` | Suppress non-essential output |
| `-y, --yes` | Skip confirmation prompts (automation-friendly) |
| `--no-input` | Alias for `--yes` |
| `--result-limit` | Limit number of results in output (0 = unlimited) |
| `--result-sort-by` | Sort output results by field |
| `--result-desc` | Sort output results in descending order |
| `--debug` | Enable debug output |
| `--config` | Config file (default: ~/.config/roam/config.yaml) |
| `--local` | Use Local API (requires Roam desktop app) |

## Shell Completions

### Bash

```bash
# macOS (Homebrew):
roam completion bash > $(brew --prefix)/etc/bash_completion.d/roam

# Linux:
roam completion bash | sudo tee /etc/bash_completion.d/roam

# Or source directly in current session:
source <(roam completion bash)
```

### Zsh

```bash
# Save to fpath:
roam completion zsh > "${fpath[1]}/_roam"

# Or add to .zshrc for auto-loading:
autoload -U compinit && compinit
```

### Fish

```bash
roam completion fish | source
```

### PowerShell

```bash
# Load for current session:
roam completion powershell | Out-String | Invoke-Expression

# Or add to profile for persistence:
roam completion powershell >> $PROFILE
```

## Development

```bash
make build
make test
make lint
```

### Future Infrastructure

- Local API auto-resolution for page-title/daily-note on writes
- Higher-level graph automation helpers
- Richer formatting options for structured outputs

## License

MIT

## Links

- https://github.com/salmonumbrella/roam-cli
- https://roamresearch.com/#/app/developer-documentation
