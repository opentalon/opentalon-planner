# opentalon-planner

[![CI](https://github.com/opentalon/opentalon-planner/actions/workflows/ci.yml/badge.svg)](https://github.com/opentalon/opentalon-planner/actions/workflows/ci.yml)

Pipeline confirmation strategy plugin for [OpenTalon](https://github.com/opentalon/opentalon).

Decides whether a multi-step pipeline requires user confirmation before execution. The embedded planner in opentalon core handles step decomposition (LLM call); this plugin only controls the confirmation gate.

## Modes

| Mode | Behavior |
|------|----------|
| `write_only` (default) | Confirm only when the pipeline contains write operations (create, update, delete, assign, etc.). Read-only pipelines (list, show, search) execute directly. |
| `always` | Every multi-step pipeline requires confirmation (opentalon's original behavior). |
| `never` | All pipelines execute without confirmation. |

## Configuration

```yaml
plugins:
  planner:
    source: github.com/opentalon/opentalon-planner
    config:
      mode: "write_only"
      skip_patterns:
        - "^list-"
        - "^show-"
        - "^search-"
        - "ask_knowledge"
      confirm_patterns:
        - "^create-"
        - "^update-"
        - "^delete-"
        - "^assign-"

orchestrator:
  pipeline:
    enabled: true
    confirmation_plugin: "planner"
    confirmation_action: "check_confirmation"
```

### Config fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `mode` | string | `"write_only"` | Confirmation strategy |
| `skip_patterns` | string[] | `["^list-", "^show-", "^search-", "^get-", "ask_knowledge"]` | Regex patterns for actions that never need confirmation |
| `confirm_patterns` | string[] | `["^create-", "^update-", "^delete-", ...]` | Regex patterns for actions that always need confirmation (overrides skip) |

## How it works

1. The opentalon orchestrator's embedded planner decomposes a user message into pipeline steps
2. Before asking the user to confirm, the orchestrator calls this plugin's `check_confirmation` action
3. The plugin inspects each step's action name against its patterns and mode
4. Returns `{"requires_confirmation": true/false, "reason": "...", "write_actions": [...]}`
5. The orchestrator either shows the confirmation prompt or executes the pipeline directly

Unknown actions default to requiring confirmation (fail-safe).

## Action

### `check_confirmation`

**Input:**
```json
{
  "steps": "[{\"plugin\":\"timly\",\"action\":\"timly__list-categories\",\"name\":\"List categories\"},{\"plugin\":\"timly\",\"action\":\"timly__create-item\",\"name\":\"Create item\"}]"
}
```

**Output:**
```json
{
  "requires_confirmation": true,
  "reason": "write operations: timly__create-item",
  "write_actions": ["timly__create-item"]
}
```

## Development

```bash
go test -v ./...
```

## License

Same as [opentalon](https://github.com/opentalon/opentalon).
