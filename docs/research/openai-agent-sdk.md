---
title: "How applications use the two OpenAI Agent SDKs"
---

Two different OpenAI products share the name "Agent SDK", and applications drive them in completely different ways. The first is the hosted Agents API, reached through `openai-python`'s `client.beta.agents`, where OpenAI runs the agent, the session and its execution environment server-side and the application submits input events and consumes an event stream. The second is [openai-agents-python](https://github.com/openai/openai-agents-python), the open-source "OpenAI Agents SDK" framework, where the application owns the loop and the framework calls the Responses API.

OpenAgentCore's `/v1` is the first product: the [coverage ledger](../../contracts/agents-api/index.md) pins `openai-python` 3.13.0 and serves exactly its 58 method and path pairs. This document records how real applications use each SDK, so OpenAgentCore developers can tell what works against `/v1`, what does not, and why.

## How to read the citations

Code claims link to the exact file at a pinned commit. Three repository states are used throughout:

- **Pinned SDK** — [openai-python](https://github.com/openai/openai-python) at commit [`d7c41efee1b0802b79f3f88a678ef2052b06e9ce`](https://github.com/openai/openai-python/tree/d7c41efee1b0802b79f3f88a678ef2052b06e9ce), version 3.13.0, the version OpenAgentCore targets ([upstream.json](../../contracts/agents-api/upstream.json)).
- **Current SDK** — openai-python v3.24.0, release tag [`v3.24.0`](https://github.com/openai/openai-python/tree/v3.24.0) (commit `637f1b8b2e9fdc3220fd4edbb8602cc89dc489c8`), used only for "newer than the pin" notes.
- **Agents SDK repo** — [openai-agents-python](https://github.com/openai/openai-agents-python) at commit [`81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3`](https://github.com/openai/openai-agents-python/tree/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3), version 0.23.1. The documentation site at <https://openai.github.io/openai-agents-python/> is generated from the same repository; claims below were read from the repository files at that commit.

Claims are labeled by how they were verified:

- **Verified against the pin** means the claim was read from the pinned commit's source.
- **Verified against the current release** means it was read from the published openai-python 3.24.0 wheel or the openai-agents-python repository at `81f0ccf` (= 0.23.1).
- **Documented** means an official documentation page states it and the source was also checked where possible.
- Anything unverified is called out in place. No code example in this document was executed; they are written from the generated type signatures and docstrings.

## Versions verified on 2026-10-04

| Product | Version | Released | Source |
| --- | --- | --- | --- |
| openai-python (pin) | 3.13.0 | 2026-09-10 | [PyPI](https://pypi.org/project/openai/3.13.0/), [upstream.json](../../contracts/agents-api/upstream.json) |
| openai-python (latest) | 3.24.0 | 2026-10-02 | [PyPI](https://pypi.org/project/openai/3.24.0/) |
| openai-agents | 0.23.1 | 2026-10-02 | [PyPI](https://pypi.org/project/openai-agents/0.23.1/) |

## Part A — `client.beta.agents`, the hosted Agents API

### What this surface is

`client.beta.agents` is the client for OpenAI's hosted Agents API: reusable Agents, managed Sessions with server-side execution environments, a live session event stream, durable Turns and Items, artifacts, files, skills, environment templates and vaults. The pinned SDK exposes 42 operations under `beta/agents` plus 5 Files operations and 11 Skills operations, the 58 pairs listed in [upstream-routes.json](../../contracts/agents-api/upstream-routes.json). Every request carries `OpenAI-Beta: agents=v1` ([upstream.json](../../contracts/agents-api/upstream.json)).

### Installation and client setup

```sh
# the pinned version
pip install openai==3.13.0
```

```python
# openai-python 3.13.0 (the pinned version)
from openai import OpenAI, AsyncOpenAI

client = OpenAI()                 # reads OPENAI_API_KEY and OPENAI_BASE_URL
client = OpenAI(base_url="https://core.example/v1", api_key="sk-...")  # explicit
async_client = AsyncOpenAI()      # async variant, same resources and parameters
```

The synchronous client infers `api_key` from `OPENAI_API_KEY` and `base_url` from `OPENAI_BASE_URL` when they are not passed explicitly ([`_client.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/_client.py)); the async client does the same ([`_client.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/_client.py)). Defaults are a 10-minute timeout and 2 retries with exponential backoff ([`_constants.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/_constants.py)).

The Beta header is not a client-level default. Every method under `client.beta.agents` and `client.beta.agents.vaults` starts with `extra_headers = {"OpenAI-Beta": "agents=v1", **(extra_headers or {})}`, so the SDK sets it per request and a caller-supplied `extra_headers` value overrides it ([`agents.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/agents.py), [`sessions.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/sessions.py), [`vaults.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/vaults/vaults.py)). The top-level `client.files` and `client.skills` resources do not set it ([`files.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/files.py), [`skills.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/skills/skills.py)), which matches the Core rule that only `/agents` and `/vaults` need the header ([Agents API guide](../api/public-agent-api.md)).

Every resource also has `.with_raw_response` and `.with_streaming_response` wrappers, and every resource class has an `Async*` counterpart ([`agents.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/agents.py), [`sessions.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/sessions.py)).

### Resources an application touches

| Resource | SDK access | Operations |
| --- | --- | --- |
| Agents | `client.beta.agents` | create, retrieve, update, list, delete |
| Sessions | `client.beta.agents.sessions` | create (JSON or `stream=True`), retrieve, update, list, delete, plus the `stream` helper |
| Session events | `client.beta.agents.sessions.events` | create (input), stream (SSE) |
| Turns | `client.beta.agents.sessions.turns` | retrieve, list |
| Items | `client.beta.agents.sessions.items` | list |
| Subagents | `client.beta.agents.sessions.subagents` | retrieve, list; `subagents.items.list`; `subagents.turns.retrieve`/`list`; `subagents.turns.items.list` |
| Environments | `client.beta.agents.environments` | retrieve |
| Environment files | `client.beta.agents.environments.files` | create (`file_id` or `inline`), list |
| Environment Templates | `client.beta.agents.environments.templates` | create, retrieve, update, list, delete |
| Vaults | `client.beta.agents.vaults` | create, retrieve, list, delete |
| Vault Credentials | `client.beta.agents.vaults.credentials` | create, retrieve, update, list, delete |
| Artifacts | `client.beta.agents.sessions.artifacts` | retrieve, list, delete, content |
| Files | `client.files` | create, retrieve, list, delete, content |
| Skills | `client.skills`, `client.skills.versions` | create, retrieve, update, list, delete, content; versions create, retrieve, list, delete, content |

The per-resource shapes were verified from the pinned resources: [agents](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/agents.py), [sessions](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/sessions.py), [events](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/events.py), [turns](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/turns.py), [items](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/items.py), [artifacts](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/artifacts.py), [subagents](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/subagents/subagents.py), [environments](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/environments/environments.py), [environment files](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/environments/files.py), [templates](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/environments/templates.py), [vaults](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/vaults/vaults.py), [credentials](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/vaults/credentials.py), [files](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/files.py), [skills](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/skills/skills.py) and [skill versions](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/skills/versions/versions.py).

Usage shapes worth knowing:

- **Agents** are reusable configuration. `create` requires `model` and accepts `instructions`, `metadata` (up to 16 pairs), `name`, `reasoning`, `service_tier`, `text`, `tools` and `multi_agent` ([`agent_create_params.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/types/beta/agent_create_params.py)); `update` changes only sent fields and `metadata` replaces all pairs ([`agent_update_params.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/types/beta/agent_update_params.py)). Function tools are declared as `{"type": "function", "name", "description", "parameters"}` ([`persisted_agent_tool_param.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/types/beta/persisted_agent_tool_param.py)).
- **Sessions** are created with a required `environment` and optional `agent_id`/inline `agent`, `input`, `metadata`, `stream` and `vault_ids` ([`sessions.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/sessions.py)). `environment` is one of `{"type": "none"}`, `{"type": "openai_hosted", ...}` or `{"type": "self_hosted", "workspace_directory": ...}` ([`environment_param.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/types/beta/environment_param.py)). With `stream=True` the POST response is itself a `Stream[AgentSessionEvent]`; otherwise it is an `AgentSession` with `status` in `idle`, `in_progress`, `requires_action`, `failed` ([`agent_session.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/types/beta/agent_session.py)). `update` accepts only `metadata` ([`session_update_params.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/types/beta/agents/session_update_params.py)).
- **Session events**: `events.create(session_id, events=[...], idempotency_key=...)` returns nothing (the server answers 202) and `events.stream(session_id)` returns a `Stream[AgentSessionEvent]` with `Accept: text/event-stream` ([`events.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/events.py)). Input events are exactly three variants: `agent.session.input.message`, `agent.session.input.cancel` and `agent.session.input.tool_result` ([`agent_session_input_param.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/types/beta/agent_session_input_param.py)). The event union has 30 members, including session status events, turn lifecycle events, text and reasoning deltas, item events and subagent events ([`agent_session_event.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/types/beta/agent_session_event.py)).
- **Turns and Items** are read with `turns.retrieve`/`list` and `items.list`, all paginated by `after`, `limit` and `order` ([`turns.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/turns.py), [`items.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/items.py)). A Turn's `status` is one of `queued`, `in_progress`, `waiting`, `completed`, `failed`, `cancelled` and its `usage` is optional ([`turn.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/types/beta/agents/sessions/turn.py), [`token_usage.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/types/beta/token_usage.py)).
- **Artifacts** are listed per session, downloaded with `content(artifact_id, session_id=...)`, which returns the SDK's binary response object, and deleted without touching the underlying workspace file ([`artifacts.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/artifacts.py)).
- **Subagents** are read-only child work: retrieve and list a subagent, list its Items, and read its Turns and their Items ([`subagents.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/subagents/subagents.py)).
- **Vault credentials** take an `auth` object and a `name`; secret values are write-only ([`credentials.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/vaults/credentials.py)).

### The end-to-end flow an application writes

```python
# openai-python 3.13.0 (the pinned version), synchronous client
from openai import OpenAI

client = OpenAI()  # reads OPENAI_API_KEY and OPENAI_BASE_URL

# 1. Save a reusable Agent with a function tool.
agent = client.beta.agents.create(
    model="your-model-id",
    name="Weather assistant",
    instructions="Answer weather questions with the get_weather tool.",
    tools=[{
        "type": "function",
        "name": "get_weather",
        "description": "Current weather for a city",
        "parameters": {"type": "object", "properties": {"city": {"type": "string"}}, "required": ["city"]},
    }],
)

# 2. Create a Session. The Agent configuration is copied into it.
session = client.beta.agents.sessions.create(
    environment={"type": "openai_hosted"},
    agent_id=agent.id,
    metadata={"ticket": "T-123"},
)

# 3. One idle turn: subscribe first, submit input, run registered tools, stop at idle/failed.
def get_weather(args: dict) -> str:
    return f"Sunny in {args['city']}"

with client.beta.agents.sessions.stream(
    session.id,
    input="What's the weather in Paris?",
    tool_handlers={"get_weather": get_weather},
) as stream:
    for event in stream:
        if event.type == "agent.session.turn.output_text.delta":
            print(event.delta, end="", flush=True)
        elif event.type == "agent.session.turn.completed":
            print(f"\nusage: {event.usage}")

# 4. Durable history after the turn.
latest = client.beta.agents.sessions.turns.list(session.id, order="desc", limit=1).data[0]
print(latest.status)
for item in client.beta.agents.sessions.items.list(session.id, order="asc"):
    print(item.type)

# 5. Download what the turn produced.
for artifact in client.beta.agents.sessions.artifacts.list(session.id):
    data = client.beta.agents.sessions.artifacts.content(artifact.id, session_id=session.id)
    data.write_to_file(artifact.path.rsplit("/", 1)[-1])
```

Step by step, with the mechanics an application has to know:

1. **Create an Agent.** `model` is required; `instructions`, `name`, `metadata`, `reasoning`, `service_tier`, `text`, `tools` and `multi_agent` are optional ([`agents.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/agents.py)).
2. **Create a Session.** The pinned SDK overloads `create` so `stream=True` types as `Stream[AgentSessionEvent]` and the default types as `AgentSession`; both use the same POST body ([`sessions.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/sessions.py)). An inline `agent` (instead of `agent_id`) overrides fields for that session.
3. **Open the event stream.** `sessions.events.stream(session_id)` returns the live SSE stream as `Stream[AgentSessionEvent]` ([`events.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/events.py)). Open it before sending input; it does not replay missed events ([Agents API guide](../api/public-agent-api.md)).
4. **Send input events.** `events.create(session_id, events=[...], idempotency_key=...)` submits messages, cancellation or tool results and returns `None` on the 202 response ([`events.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/events.py)). The three input variants are message, cancel and tool result ([`agent_session_input_param.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/types/beta/agent_session_input_param.py)).
5. **Handle `requires_action` and function calls.** When the model calls a function, the session reports `requires_action` and a `function_call` required action with `call_id`, `turn_id`, `name` and `arguments` ([`agent_session.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/types/beta/agent_session.py)). The same call also arrives on the stream as an `agent.session.turn.item.added` event whose item is a `function_call` ([`agent_function_call_item.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/types/beta/agent_function_call_item.py)). Submit the result with `agent.session.input.tool_result`, `success` plus either `output` (a string or a list of input content parts) or `error` ([`agent_function_call_output_param.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/types/beta/agent_function_call_output_param.py)).
6. **Use `sessions.stream` to automate the loop.** The helper is the convenience most small applications use; its exact behavior is described in the next section.
7. **Read Turns and Items** for durable history and recovery, and **download Artifacts** produced by completed turns ([`artifacts.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/artifacts.py)).

Without the helper, an application drives the same flow by hand:

```python
# openai-python 3.13.0 — manual tool loop
import json
import uuid

def call_arguments(action) -> dict:
    # AgentFunctionCallItem.arguments is object: a dict or a JSON string.
    return action.arguments if isinstance(action.arguments, dict) else json.loads(action.arguments)

with client.beta.agents.sessions.events.stream(session.id) as stream:
    client.beta.agents.sessions.events.create(
        session.id,
        events=[{
            "type": "agent.session.input.message",
            "input": [{"role": "user", "content": [{"type": "input_text", "text": "What's the weather in Paris?"}]}],
        }],
        idempotency_key=str(uuid.uuid4()),
    )
    for event in stream:
        if event.type == "agent.session.requires_action":
            for action in event.session.required_actions:
                if action.type == "function_call":
                    client.beta.agents.sessions.events.create(
                        session.id,
                        events=[{
                            "type": "agent.session.input.tool_result",
                            "turn_id": action.turn_id,
                            "call_id": action.call_id,
                            "success": True,
                            "output": get_weather(call_arguments(action)),
                        }],
                    )
        elif event.type == "agent.session.idle":
            break
```

More complete flows — async usage, streamed creation and cancellation, files and Artifacts, Skills and templates, Vaults and MCP, self-hosted Sessions, and typed errors — are in the [Agents API guide](../api/public-agent-api.md#worked-examples) and the runnable [`example/hosted-agents-python`](../../example/hosted-agents-python) scripts.

### SDK conveniences and gotchas

**Pagination.** List methods return `SyncCursorPage` / `AsyncPaginator` (async); pages expose `.data` and `.has_more` and auto-paginate when iterated, using the last item's ID as the next `after` cursor ([`pagination.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/pagination.py), [`turns.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/turns.py)). One list differs: `environments.files.list` returns `SyncTokenPage[EnvironmentFile]`, which pages with an opaque `.next` token instead of `after` ([`files.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/environments/files.py), [`pagination.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/pagination.py)).

**Idempotency-Key.** Only two call sites take a named `idempotency_key`: `sessions.events.create` and the `sessions.stream` helper ([`events.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/events.py), [`sessions.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/sessions.py)). `sessions.create` has no such parameter; applications pass `extra_headers={"Idempotency-Key": ...}`, which is also what the OpenAgentCore guide shows ([Agents API guide](../api/public-agent-api.md)). The helper generates a `uuid4` key for the input submission when none is given, and a fresh key for each tool result; it strips any `Idempotency-Key` from the shared request options so an input key is never reused for a tool result ([`_streams.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/lib/streaming/agents/_streams.py)). It retries the known "Unknown pending tool call" registration race at 0.1 s, 0.3 s and 0.6 s ([`_streams.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/lib/streaming/agents/_streams.py), with the race predicate in [`_tools.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/lib/streaming/agents/_tools.py)).

**The `sessions.stream` helper in detail.** It requires an idle session and raises `ValueError` otherwise; it subscribes to the event stream *before* submitting input; it accepts a single writer only; handlers are looked up by function name and receive the parsed arguments dict; registered handlers run sequentially after the call event is yielded; unregistered tools are left to the caller; a handler exception submits `success: false` with the generic message "Tool handler failed." and never the exception text; iteration stops on `agent.session.failed` or on `agent.session.idle` after the turn ended; an early stream end raises `RuntimeError`; and `close()` does not cancel the backend turn ([`_streams.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/lib/streaming/agents/_streams.py)).

**`extra_body`.** Every generated method accepts `extra_body`, which the SDK merges into the JSON body last. This is the supported way to send OpenAgentCore's `x_agents_core` fields, as the Core guide shows ([Agents API guide](../api/public-agent-api.md)); the pinned SDK itself has no `x_agents_core` parameter.

**Streaming context managers.** Both `events.stream` and `sessions.stream` are used as `with ... as stream:` (or `async with`); the returned `Stream`/`AsyncStream` supports iteration and `close()` ([`events.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/events.py), [`_streams.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/lib/streaming/agents/_streams.py)).

**Sync/async parity.** Every resource ships `Agents`/`AsyncAgents`, `Sessions`/`AsyncSessions` and so on with identical parameters; the async stream helper accepts both plain and awaitable handler results ([`sessions.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/sessions.py), [`_streams.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/lib/streaming/agents/_streams.py)).

**Client-side vs server-side.** The server runs the agent, executes the harness and owns Turns, Items and Environments. The SDK is a transport plus the `sessions.stream` tool-call loop; it never executes tools itself and never runs an agent loop outside that helper. The helper's "loop" is only subscribe → yield → submit tool results → stop at terminal events.

**Retries.** The SDK retries connection errors and 408/409/429/5xx responses up to `max_retries` (default 2) with exponential backoff ([`_constants.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/_constants.py)); retried `events.create` calls reuse the same `Idempotency-Key` only if the caller supplied one, which is why the helper generates keys.

### Version notes: pin 3.13.0 vs current 3.24.0

The pin is 3.13.0, released 2026-09-10; the latest release at the time of writing is 3.24.0, released 2026-10-02 ([PyPI openai](https://pypi.org/project/openai/)). Everything in this section is **newer than the pin** and must not be used against the pinned baseline; OpenAgentCore's contract tests reject operations and fields outside the pin ([coverage ledger](../../contracts/agents-api/index.md)).

- `sessions.traces.list(session_id)` is a new resource that lists published root-turn traces as OTLP JSON, ordered by turn creation time, with `after`/`limit`/`order` ([`traces.py`](https://github.com/openai/openai-python/blob/v3.24.0/src/openai/resources/beta/agents/sessions/traces.py)).
- `sessions.create` gained an `output_type: type[OutputT] | None` parameter ([`sessions.py`](https://github.com/openai/openai-python/blob/v3.24.0/src/openai/resources/beta/agents/sessions/sessions.py)).
- Session input gained `agent.session.input.computer_use_approval_request_result` with browser authentication cancel/submit and origin-access responses ([`agent_session_input_param.py`](https://github.com/openai/openai-python/blob/v3.24.0/src/openai/types/beta/agent_session_input_param.py)).
- A new `agent.session.environment.reset` event reports a replaced hosted sandbox; conversation history survives but files and processes do not ([`agent_session_environment_reset_event.py`](https://github.com/openai/openai-python/blob/v3.24.0/src/openai/types/beta/agent_session_environment_reset_event.py)).
- `artifacts.for_result(...)` and environment-file `prepare`/`prepare_directory`/`upload` helpers were added ([`artifacts.py`](https://github.com/openai/openai-python/blob/v3.24.0/src/openai/resources/beta/agents/sessions/artifacts.py), [`files.py`](https://github.com/openai/openai-python/blob/v3.24.0/src/openai/resources/beta/agents/environments/files.py)).

I compared the resource file lists and method names between the pin and the 3.24.0 wheel and found no pinned operation removed; the current release appears to be a superset for `beta/agents`. I did not exhaustively diff every type field, so an added optional field elsewhere is possible.

## Part B — openai-agents-python, the client-side Agents SDK

### Installation and hello world

```sh
pip install openai-agents
```

The package requires Python 3.10 or newer ([README](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/README.md)) and depends on `openai>=3.0.0,<4` ([pyproject.toml](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/pyproject.toml)).

```python
# openai-agents 0.23.1
from agents import Agent, Runner

agent = Agent(name="Assistant", instructions="You are a helpful assistant")

result = Runner.run_sync(agent, "Write a haiku about recursion in programming.")
print(result.final_output)
```

`Runner.run` is the async entry point, `Runner.run_sync` wraps it for synchronous callers (and cannot be used inside a running event loop), and `Runner.run_streamed` returns a `RunResultStreaming` whose `stream_events()` yields semantic stream events ([`run.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/run.py), [`result.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/result.py)). A `RunResult` exposes `final_output`, `new_items`, `raw_responses`, `last_agent` and `to_input_list()` for feeding a later run ([`result.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/result.py)). The hello-world above is exactly the documented first example ([docs/index.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/index.md), [quickstart](https://openai.github.io/openai-agents-python/quickstart/)).

### Which OpenAI API it calls

**Verified against the current release:** the framework does not call the hosted Agents API. Its default OpenAI model, `OpenAIResponsesModel`, calls `client.responses.create(...)` for non-streaming requests and `client.responses.with_streaming_response.create(...)` for streaming ones ([`openai_responses.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/models/openai_responses.py)). A grep of `src/` for `beta.agents`, `beta/agents` or `agents=v1` returns nothing. The alternative `OpenAIChatCompletionsModel` calls `client.chat.completions.create(...)` ([`openai_chatcompletions.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/models/openai_chatcompletions.py)).

Consequently, the framework can point at a server that implements the Responses API (or Chat Completions), but not at a server that only implements the hosted Agents API. Driving the hosted Agents API from the framework would require a custom `Model` implementation; the `Model` interface is abstract (`get_response`, `stream_response`) and the SDK ships no `beta.agents` adapter ([`interface.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/models/interface.py)).

The default provider is `MultiProvider`, which routes unprefixed model names (or an `openai/` prefix) to `OpenAIProvider` and can route `litellm/` and `any-llm/` prefixes elsewhere ([`multi_provider.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/models/multi_provider.py), [`run_config.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/run_config.py)). `OpenAIProvider` resolves a model name to `OpenAIResponsesModel` unless `use_responses=False` ([`openai_provider.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/models/openai_provider.py)).

Configuration surfaces, all **verified against the current release**:

```python
# openai-agents 0.23.1
from openai import AsyncOpenAI
from agents import (
    set_default_openai_client,
    set_default_openai_key,
    set_default_openai_api,
)

set_default_openai_key("sk-...")                                  # instead of OPENAI_API_KEY
set_default_openai_client(AsyncOpenAI(base_url="https://gateway.example/v1", api_key="..."))
set_default_openai_api("chat_completions")                        # default is "responses"
```

- `set_default_openai_key(key, use_for_tracing=True)` and `set_default_openai_client(client, use_for_tracing=True)` are the public wrappers; the second argument controls whether the key/client is also used for trace export ([`__init__.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/__init__.py), [`_config.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/_config.py)).
- When the provider constructs its own client it passes the SDK-wide default key (if one was set) and `os.getenv("OPENAI_BASE_URL")`; the openai SDK then fills in `OPENAI_API_KEY` from the environment when no key was set, and `OPENAI_WEBSOCKET_BASE_URL` is used for Responses-over-WebSocket ([`openai_provider.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/models/openai_provider.py), [docs/config.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/config.md)).
- Passing `openai_client=` to `OpenAIProvider` cannot be combined with `api_key`, `base_url`, `websocket_base_url`, `organization` or `project`; the provider raises `UserError` ([`openai_provider.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/models/openai_provider.py)).
- `RunConfig(model_provider=...)` overrides the provider for one run, and `RunConfig(model=...)` sets a default model ([`run_config.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/run_config.py)).

### The building blocks applications use

**Agent.** `Agent` is a dataclass with `name`, `instructions` (string or a callable returning a string), `model` (string or `Model`), `tools`, `mcp_servers`, `handoffs`, `input_guardrails`, `output_guardrails`, `output_type`, `model_settings`, `hooks` and `tool_use_behavior` ([`agent.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/agent.py)). If `model` is omitted the SDK uses `get_default_model()`, which is `OPENAI_DEFAULT_MODEL` or `gpt-5.6-luna` in 0.23.1, with GPT-5 default settings ([`default_models.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/models/default_models.py), [docs/models/index.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/models/index.md)).

**Function tools.** `@function_tool` wraps a Python function; the schema is generated from the signature, docstring and type hints, with Pydantic used for validation ([`function_schema.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/function_schema.py), [docs/tools.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/tools.md)). `tool` is a public alias for `function_tool` ([`decorators.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/decorators.py)). The decorator also accepts `name_override`, `description_override`, `strict_mode`, `is_enabled`, `needs_approval`, timeouts and tool guardrails ([`tool.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/tool.py)).

```python
# openai-agents 0.23.1
from pydantic import BaseModel
from agents import Agent, function_tool

class WeatherArgs(BaseModel):
    city: str

@function_tool
def get_weather(args: WeatherArgs) -> str:
    """Current weather for a city.

    Args:
        args: The city to look up.
    """
    return f"Sunny in {args.city}"

agent = Agent(name="Weather", tools=[get_weather])
```

**Hosted tools.** The SDK ships `WebSearchTool`, `FileSearchTool`, `ComputerTool`, `CodeInterpreterTool`, `ImageGenerationTool`, `HostedMCPTool`, `ToolSearchTool` and `ProgrammaticToolCallingTool` ([`tool.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/tool.py), [docs/tools.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/tools.md)). The hosted tools are documented as only usable with OpenAI models through the Responses API ([docs/tools.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/tools.md)); `HostedMCPTool` is a remote MCP server that the Responses API calls directly, while local MCP servers are attached with `Agent(mcp_servers=[...])` using `MCPServerStdio`, `MCPServerSse` or `MCPServerStreamableHttp` ([docs/mcp.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/mcp.md), [`mcp/server.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/mcp/server.py)).

**Handoffs.** Passing agents in `Agent.handoffs` or wrapping them with `handoff()` turns each destination into a tool (default name `transfer_to_<agent_name>`); `on_handoff`, `input_type`, `input_filter` and `is_enabled` customize the handoff ([`handoffs/__init__.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/handoffs/__init__.py), [docs/handoffs.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/handoffs.md)). The alternative orchestration pattern is `Agent.as_tool()` ([`agent.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/agent.py), [docs/multi_agent.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/multi_agent.md)).

**Guardrails.** `@input_guardrail` and `@output_guardrail` decorate functions returning `GuardrailFunctionOutput(tripwire_triggered=...)`; input guardrails run only for the first agent and output guardrails only for the agent producing the final output. Tripwires raise `InputGuardrailTripwireTriggered` / `OutputGuardrailTripwireTriggered` ([`guardrail.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/guardrail.py), [docs/guardrails.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/guardrails.md)).

**Sessions and memory.** Passing `session=` to `Runner.run` makes the SDK load history before the run and persist new items after it; `SQLiteSession("conversation_123")` is in-memory by default and file-backed when given a path ([`sqlite_session.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/memory/sqlite_session.py), [docs/sessions/index.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/sessions/index.md)). The extensions package adds SQLAlchemy, Redis, MongoDB, Dapr and encrypted session backends ([`extensions/memory`](https://github.com/openai/openai-agents-python/tree/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/extensions/memory)).

**Run context.** `Runner.run(..., context=obj)` passes `obj` into tools, hooks, guardrails and handoff callbacks as `RunContextWrapper[T].context`; the wrapper also exposes run `usage` ([`run_context.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/run_context.py), [docs/context.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/context.md)).

**Structured outputs.** `Agent(output_type=SomeModel)` makes `result.final_output` an instance of that type; the default output type is `str` ([`agent.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/agent.py), [docs/results.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/results.md)).

**Model settings and providers.** `ModelSettings` tunes temperature, top_p, reasoning effort/verbosity and more; GPT-5 models get SDK defaults per model name ([`model_settings.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/model_settings.py), [`default_models.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/models/default_models.py)). Non-OpenAI providers are reached through the `LitellmModel` / `LitellmProvider` and `AnyLLMModel` adapters or a custom `ModelProvider` ([`extensions/models`](https://github.com/openai/openai-agents-python/tree/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/extensions/models), [docs/models/index.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/models/index.md)).

**Tracing.** Tracing is on by default. Applications disable it globally with the `OPENAI_AGENTS_DISABLE_TRACING=1` environment variable or `set_tracing_disabled(True)`, or per run with `RunConfig(tracing_disabled=True)`; they replace or add exporters with `set_trace_processors([...])` / `add_trace_processor(...)`, and set the export key with `set_tracing_export_api_key(...)` ([`tracing/__init__.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/tracing/__init__.py), [docs/tracing.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/tracing.md)). The default exporter sends traces to the OpenAI platform, so a self-hosted deployment must disable tracing or install a custom processor.

**Version note.** All framework claims above are verified against 0.23.1 (repository commit `81f0ccf`, released 2026-10-02). Earlier versions had different defaults and surfaces (for example, the package's `openai>=3.0.0,<4` floor is documented as a 0.21.0 change in [docs/config.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/config.md)); I did not verify behavior before 0.23.1.

## Part C — How the two relate

| | Hosted Agents API (`client.beta.agents`) | Agents SDK framework (`openai-agents`) |
| --- | --- | --- |
| What it is | SDK for OpenAI's hosted Agents service | Open-source framework for building agents in your app |
| API called | `/v1/agents/*`, `/v1/vaults/*`, `/v1/files`, `/v1/skills` | `/v1/responses` (default) or `/v1/chat/completions` |
| Who runs the loop | The service; the client subscribes and submits tool results | Your process; `Runner` loops until final output |
| State | Server-side Sessions, Turns, Items, Environments | Client-side history: `to_input_list()`, `session=`, or OpenAI-managed `previous_response_id` / `conversation_id` |
| Tools | Declared on the Agent; the harness executes them, your code returns results | Python functions run in your process; hosted tools run in the Responses API |
| Typical client | `openai` 3.13.0 (pinned), `client.beta.agents` | `openai-agents` 0.23.1 |

These are two different products that both put "Agent" in the name. The hosted Agents API is a server-side product: you save an Agent, start a Session, and interact with a managed execution environment through an event stream ([Agents API guide](../api/public-agent-api.md)). The Agents SDK framework is a client-side library: you construct `Agent` objects in code and `Runner` calls the Responses API, dispatching your Python tools in a loop ([docs/index.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/index.md)). The framework's own documentation draws this line as "Agents SDK or Responses API?" and notes that the SDK wraps model calls in a higher-level runtime ([docs/index.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/index.md)).

**OpenAgentCore's `/v1` corresponds to the hosted Agents API.** The [coverage ledger](../../contracts/agents-api/index.md) states Core targets the complete OpenAI Agents API pinned to `openai-python` 3.13.0 and serves exactly its 58 routes, with Core-only additions inside `x_agents_core` ([docs/api/index.md](../api/index.md)). An application that talks to `/v1` should therefore use `client.beta.agents` (and `client.files` / `client.skills`), not the framework.

What that implies in practice:

- An application built on the pinned `client.beta.agents` works against `/v1` for the 58 pinned routes, subject to Core's documented differences ([coverage ledger](../../contracts/agents-api/index.md)). The SDK sets the Beta header automatically; plain HTTP clients must set `OpenAI-Beta: agents=v1` on `/agents` and `/vaults` ([Agents API guide](../api/public-agent-api.md)).
- An application built on `openai-agents` cannot simply point `OPENAI_BASE_URL` at `/v1`. The framework calls `/responses`, which Core does not serve; `/v1` only has the pinned Agents, Files and Skills routes ([upstream-routes.json](../../contracts/agents-api/upstream-routes.json)). Hosted tools and default tracing also target OpenAI services, not a self-hosted Core.
- A custom bridge is theoretically possible: an application could implement `agents.models.interface.Model` on top of `client.beta.agents` so the framework's `Runner` submits session events instead of Responses requests, but the SDK ships no such adapter and the tool-call loop, guardrails and handoffs would then be split between the framework and the hosted harness. Nothing in OpenAgentCore or the framework currently does this.
- Core's `x_agents_core.model_provider` `responses` protocol is unrelated to this choice. It names the protocol a Harness uses to call a model endpoint (Codex's native protocol is `responses`) ([model execution](../../contracts/agents-api/model-execution.md)); it does not make Core's `/v1` a Responses API.

## What this means for OpenAgentCore

- **Supported client.** Applications should install `openai==3.13.0` and use `client.beta.agents`, `client.files` and `client.skills` ([upstream.json](../../contracts/agents-api/upstream.json), [Agents API guide](../api/public-agent-api.md)). Core's contract tests hold the router and schema to that pin, and newer SDK operations or fields wait for a protocol upgrade ([coverage ledger](../../contracts/agents-api/index.md)).
- **Header.** Core requires `OpenAI-Beta: agents=v1` on `/agents` and `/vaults`; the pinned SDK adds it per request, and the Files and Skills routes need no Beta header ([Agents API guide](../api/public-agent-api.md), [`files.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/files.py)).
- **Core extensions.** `harness`, `model_provider`, `harness_config`, `environment` and `installation` travel in `x_agents_core` through the SDK's `extra_body`; Core rejects any other member ([Agents API guide](../api/public-agent-api.md)).
- **Tool loop.** The pinned SDK's `sessions.stream(..., tool_handlers=...)` helper is the intended convenience for function tools and matches Core's model: subscribe first, submit input, return results, stop at `idle` or `failed`. The helper's preconditions (idle session, one writer) are the application's responsibility.
- **Idempotency.** Core returns the original Session when a creation retry reuses the same `Idempotency-Key`; the official service creates a new Session, and Core credential creation takes no key at all ([coverage ledger](../../contracts/agents-api/index.md)). Applications must send the key on `sessions.create` through `extra_headers`, since the pinned SDK has no named parameter there.
- **Streams.** The event stream is live-only; applications recover gaps from Turns and Items ([Agents API guide](../api/public-agent-api.md)). The SDK helper deliberately does not replay missed events, matching Core.
- **Framework expectations.** If a user asks whether `openai-agents` can drive OpenAgentCore, the accurate answer is no: it calls the Responses API, and `/v1` implements the hosted Agents API. The closest supported experience is `client.beta.agents` plus the `sessions.stream` tool-handler loop; the framework's handoffs, guardrails and client-side sessions do not map one-to-one to `/v1` features (Core's Subagents are server-side child work, not the framework's `handoff()` primitive).

## Sources

Pinned SDK — openai-python 3.13.0, commit `d7c41efee1b0802b79f3f88a678ef2052b06e9ce` (<https://github.com/openai/openai-python/tree/d7c41efee1b0802b79f3f88a678ef2052b06e9ce>):

- [src/openai/resources/beta/agents/agents.py](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/agents.py)
- [src/openai/resources/beta/agents/sessions/sessions.py](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/sessions.py)
- [src/openai/resources/beta/agents/sessions/events.py](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/events.py)
- [src/openai/resources/beta/agents/sessions/turns.py](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/turns.py)
- [src/openai/resources/beta/agents/sessions/items.py](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/items.py)
- [src/openai/resources/beta/agents/sessions/artifacts.py](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/artifacts.py)
- [src/openai/resources/beta/agents/sessions/subagents/subagents.py](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/subagents/subagents.py)
- [src/openai/resources/beta/agents/environments/environments.py](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/environments/environments.py)
- [src/openai/resources/beta/agents/environments/files.py](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/environments/files.py)
- [src/openai/resources/beta/agents/environments/templates.py](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/environments/templates.py)
- [src/openai/resources/beta/agents/vaults/vaults.py](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/vaults/vaults.py)
- [src/openai/resources/beta/agents/vaults/credentials.py](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/vaults/credentials.py)
- [src/openai/resources/files.py](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/files.py)
- [src/openai/resources/skills/skills.py](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/skills/skills.py) and [versions/versions.py](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/skills/versions/versions.py)
- [src/openai/lib/streaming/agents/_streams.py](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/lib/streaming/agents/_streams.py) and [_tools.py](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/lib/streaming/agents/_tools.py)
- [src/openai/types/beta/agent_session_input_param.py](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/types/beta/agent_session_input_param.py), [agent_session_event.py](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/types/beta/agent_session_event.py), [agent_session.py](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/types/beta/agent_session.py), [agent_function_call_item.py](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/types/beta/agent_function_call_item.py), [agent_function_call_output_param.py](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/types/beta/agent_function_call_output_param.py), [agents/sessions/turn.py](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/types/beta/agents/sessions/turn.py), [token_usage.py](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/types/beta/token_usage.py), [environment_param.py](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/types/beta/environment_param.py), [agent.py](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/types/beta/agent.py), [persisted_agent_tool_param.py](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/types/beta/persisted_agent_tool_param.py), [agent_create_params.py](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/types/beta/agent_create_params.py), [agent_update_params.py](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/types/beta/agent_update_params.py), [agents/session_update_params.py](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/types/beta/agents/session_update_params.py)
- [src/openai/_client.py](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/_client.py), [src/openai/_constants.py](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/_constants.py), [src/openai/_exceptions.py](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/_exceptions.py)
- [api.md](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/api.md) (generated method index)
- PyPI: <https://pypi.org/project/openai/3.13.0/>

Current SDK — openai-python 3.24.0, tag `v3.24.0` (commit `637f1b8b2e9fdc3220fd4edbb8602cc89dc489c8`), used only for newer-than-pin notes:

- [src/openai/resources/beta/agents/sessions/traces.py](https://github.com/openai/openai-python/blob/v3.24.0/src/openai/resources/beta/agents/sessions/traces.py), [sessions.py](https://github.com/openai/openai-python/blob/v3.24.0/src/openai/resources/beta/agents/sessions/sessions.py), [environments/files.py](https://github.com/openai/openai-python/blob/v3.24.0/src/openai/resources/beta/agents/environments/files.py), [sessions/artifacts.py](https://github.com/openai/openai-python/blob/v3.24.0/src/openai/resources/beta/agents/sessions/artifacts.py)
- [src/openai/types/beta/agent_session_input_param.py](https://github.com/openai/openai-python/blob/v3.24.0/src/openai/types/beta/agent_session_input_param.py), [agent_session_environment_reset_event.py](https://github.com/openai/openai-python/blob/v3.24.0/src/openai/types/beta/agent_session_environment_reset_event.py)
- PyPI: <https://pypi.org/project/openai/3.24.0/>; release data read from the [PyPI JSON API](https://pypi.org/pypi/openai/json)

Agents SDK — openai-agents-python 0.23.1, commit `81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3`:

- [pyproject.toml](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/pyproject.toml), [src/agents/version.py](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/version.py), [README.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/README.md)
- [src/agents/agent.py](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/agent.py), [run.py](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/run.py), [result.py](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/result.py), [run_config.py](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/run_config.py), [run_context.py](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/run_context.py), [__init__.py](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/__init__.py), [_config.py](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/_config.py)
- [src/agents/models/openai_responses.py](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/models/openai_responses.py), [openai_chatcompletions.py](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/models/openai_chatcompletions.py), [openai_provider.py](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/models/openai_provider.py), [multi_provider.py](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/models/multi_provider.py), [default_models.py](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/models/default_models.py), [interface.py](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/models/interface.py)
- [src/agents/tool.py](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/tool.py), [decorators.py](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/decorators.py), [function_schema.py](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/function_schema.py), [guardrail.py](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/guardrail.py), [handoffs/__init__.py](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/handoffs/__init__.py), [memory/sqlite_session.py](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/memory/sqlite_session.py), [mcp/server.py](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/mcp/server.py), [tracing/__init__.py](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/tracing/__init__.py), [extensions/models](https://github.com/openai/openai-agents-python/tree/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/extensions/models), [extensions/memory](https://github.com/openai/openai-agents-python/tree/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/extensions/memory)
- Docs: [index.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/index.md), [quickstart.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/quickstart.md), [config.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/config.md), [tools.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/tools.md), [mcp.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/mcp.md), [sessions/index.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/sessions/index.md), [tracing.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/tracing.md), [guardrails.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/guardrails.md), [models/index.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/models/index.md), [context.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/context.md), [multi_agent.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/multi_agent.md), [handoffs.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/handoffs.md), [results.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/results.md)
- Documentation site: <https://openai.github.io/openai-agents-python/>
- PyPI: <https://pypi.org/project/openai-agents/0.23.1/>; release data read from the [PyPI JSON API](https://pypi.org/pypi/openai-agents/json)

OpenAgentCore (this repository):

- [contracts/agents-api/upstream.json](../../contracts/agents-api/upstream.json), [upstream-routes.json](../../contracts/agents-api/upstream-routes.json), [coverage ledger](../../contracts/agents-api/index.md)
- [docs/api/public-agent-api.md](../api/public-agent-api.md), [docs/api/index.md](../api/index.md)
- [contracts/agents-api/model-execution.md](../../contracts/agents-api/model-execution.md)
