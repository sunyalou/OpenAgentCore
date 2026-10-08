# Hosted Agents Python scripts

Small standalone scripts that exercise the hosted Agents API through the pinned official SDK. They accompany the [Agents API guide](../../docs/api/public-agent-api.md) and its [worked examples](../../docs/api/public-agent-api.md#worked-examples); each script prints the resources it creates so you can inspect them afterwards.

## Requirements

- Python 3.10 or newer.
- A running Core installation, a Project API key, and a model your installation serves.

```sh
python3 -m venv .venv
. .venv/bin/activate
pip install -r requirements.txt
export OPENAI_BASE_URL='https://core.example/v1'
export OPENAI_API_KEY='<project-api-key>'
export OAC_MODEL='<model-id>'
```

## Scripts

| Script | What it shows | Extra environment |
| --- | --- | --- |
| [`quickstart.py`](quickstart.py) | An Agent with a function tool, one Turn through `sessions.stream` with `tool_handlers`, then Turns and Items | |
| [`async_quickstart.py`](async_quickstart.py) | The same flow with `AsyncOpenAI` and an async handler | |
| [`streaming.py`](streaming.py) | The `stream=True` creation stream, then a live Turn cancelled after its first delta | |
| [`files_and_artifacts.py`](files_and_artifacts.py) | Source File upload, workspace files, waiting for the Environment, Artifact download | |
| [`skills_and_template.py`](skills_and_template.py) | Skill upload and version, Environment Template, Session from the template | |
| [`vault_mcp.py`](vault_mcp.py) | Vault, `static_bearer` Credential and an HTTP MCP tool | `OAC_MCP_URL`, `OAC_MCP_TOKEN`; optional `OAC_MCP_PROMPT` |
| [`self_hosted.py`](self_hosted.py) | A `self_hosted` Session and its installation command | `OAC_WORKSPACE`, `OAC_MODEL_BASE_URL`, `OAC_MODEL_API_KEY`; optional `OAC_HARNESS`, `OAC_MODEL_PROTOCOL` |
| [`harness_and_model.py`](harness_and_model.py) | A saved Agent with an explicit harness and model, then one Turn | optional `OAC_HARNESS` (default `codex`) |
| [`inline_agent_override.py`](inline_agent_override.py) | A Session-level inline agent overriding a saved Agent | optional `OAC_HARNESS` (default `codex`) |
| [`patterns.py`](patterns.py) | Pagination, repeated creation with one `Idempotency-Key`, typed errors | |

## Notes

- The scripts create real resources and print their IDs; they do not delete them. Remove them through the API or the Web console when you are done.
- `openai_hosted` Sessions need sandbox capacity on the installation. `self_hosted.py` needs a machine you control, and `vault_mcp.py` needs a reachable HTTPS MCP server.
- `self_hosted.py` defaults to Codex with the `responses` protocol; the protocol must match the harness. Claude SDK uses `anthropic`, and MiniMax Code accepts `anthropic`, `responses` or `chat_completions` ([model execution](../../contracts/agents-api/model-execution.md)).
- `vault_mcp.py` uses the `environment` MCP origin, which runs on `openai_hosted`; a `service` origin requires a `none` placement ([MCP connection origin](../../contracts/agents-api/environments.md#public-mcp-connection-origin)).
- `files_and_artifacts.py` writes the downloaded Artifact into the current directory.

## Validation

The scripts were syntax-checked and their calls cross-checked against the pinned SDK (`openai==3.13.0`); they were not executed against a live installation in this repository.
