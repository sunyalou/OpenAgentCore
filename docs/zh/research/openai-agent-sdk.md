---
title: "应用如何使用两个 OpenAI Agent SDK"
source: docs/research/openai-agent-sdk.md
source_hash: 7f9aba1e61854f1db9e7d9174356b01a4ad6d983eb89b6bab819a74a5b0e9ab9
---

两个不同的 OpenAI 产品共用 "Agent SDK" 这一名称，应用驱动它们的方式也完全不同。第一个是托管 Agents API，通过 `openai-python` 的 `client.beta.agents` 访问：OpenAI 在服务端运行 agent、session 及其执行环境，应用提交输入事件并消费事件流。第二个是 [openai-agents-python](https://github.com/openai/openai-agents-python)，即开源的 "OpenAI Agents SDK" 框架：应用自己掌控循环，框架调用 Responses API。

OpenAgentCore 的 `/v1` 对应第一个产品：[coverage ledger](../../../contracts/agents-api/zh/index.md) 固定 `openai-python` 3.13.0，并精确提供其 58 个方法与路径对。本文记录真实应用如何使用这两个 SDK，以便 OpenAgentCore 开发者判断哪些用法能在 `/v1` 上工作、哪些不能，以及原因。

## 如何阅读引用 {#how-to-read-the-citations}

代码结论链接到固定提交中的确切文件。全文使用三种仓库状态：

- **固定 SDK（Pinned SDK）** —— 位于 commit [`d7c41efee1b0802b79f3f88a678ef2052b06e9ce`](https://github.com/openai/openai-python/tree/d7c41efee1b0802b79f3f88a678ef2052b06e9ce) 的 [openai-python](https://github.com/openai/openai-python)，版本 3.13.0，即 OpenAgentCore 的目标版本（[upstream.json](../../../contracts/agents-api/upstream.json)）。
- **当前 SDK（Current SDK）** —— openai-python v3.24.0，release tag [`v3.24.0`](https://github.com/openai/openai-python/tree/v3.24.0)（commit `637f1b8b2e9fdc3220fd4edbb8602cc89dc489c8`），仅用于 "晚于固定版本" 的说明。
- **Agents SDK 仓库** —— 位于 commit [`81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3`](https://github.com/openai/openai-agents-python/tree/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3) 的 [openai-agents-python](https://github.com/openai/openai-agents-python)，版本 0.23.1。文档站点 <https://openai.github.io/openai-agents-python/> 由同一仓库生成；以下结论读取自该提交的仓库文件。

结论按验证方式标注：

- **已对照固定版本验证（Verified against the pin）** 表示结论读取自固定提交的源码。
- **已对照当前版本验证（Verified against the current release）** 表示结论读取自已发布的 openai-python 3.24.0 wheel，或 `81f0ccf`（= 0.23.1）处的 openai-agents-python 仓库。
- **官方文档记载（Documented）** 表示官方文档页面有此说明，并尽可能核对了源码。
- 任何未验证的内容都会就地标注。本文没有执行任何代码示例；它们依据生成的类型签名和 docstring 编写。

## 于 2026-10-04 验证的版本 {#versions-verified-on-2026-10-04}

| 产品 | 版本 | 发布日期 | 来源 |
| --- | --- | --- | --- |
| openai-python（固定） | 3.13.0 | 2026-09-10 | [PyPI](https://pypi.org/project/openai/3.13.0/)、[upstream.json](../../../contracts/agents-api/upstream.json) |
| openai-python（最新） | 3.24.0 | 2026-10-02 | [PyPI](https://pypi.org/project/openai/3.24.0/) |
| openai-agents | 0.23.1 | 2026-10-02 | [PyPI](https://pypi.org/project/openai-agents/0.23.1/) |

## 第一部分 —— `client.beta.agents`，托管 Agents API {#part-a-—-client-beta-agents-the-hosted-agents-api}

### 这个接口面是什么 {#what-this-surface-is}

`client.beta.agents` 是 OpenAI 托管 Agents API 的客户端：可复用的 Agent、带服务端执行环境的托管 Session、实时 session 事件流、持久化的 Turn 和 Item，以及 artifacts、files、skills、environment templates 和 vaults。固定版本 SDK 在 `beta/agents` 下暴露 42 个操作，加上 5 个 Files 操作和 11 个 Skills 操作，共 58 个方法/路径对，见 [upstream-routes.json](../../../contracts/agents-api/upstream-routes.json)。每个请求都携带 `OpenAI-Beta: agents=v1`（[upstream.json](../../../contracts/agents-api/upstream.json)）。

### 安装与客户端配置 {#installation-and-client-setup}

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

未显式传入时，同步客户端从 `OPENAI_API_KEY` 推断 `api_key`、从 `OPENAI_BASE_URL` 推断 `base_url`（[`_client.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/_client.py)）；异步客户端同样如此（[`_client.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/_client.py)）。默认超时为 10 分钟，并以指数退避重试 2 次（[`_constants.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/_constants.py)）。

Beta 请求头不是客户端级默认值。`client.beta.agents` 和 `client.beta.agents.vaults` 下的每个方法都以 `extra_headers = {"OpenAI-Beta": "agents=v1", **(extra_headers or {})}` 开始，因此 SDK 按请求设置它，调用方提供的 `extra_headers` 会覆盖它（[`agents.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/agents.py)、[`sessions.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/sessions.py)、[`vaults.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/vaults/vaults.py)）。顶层的 `client.files` 和 `client.skills` 资源不设置该请求头（[`files.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/files.py)、[`skills.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/skills/skills.py)），这与 Core 的规则一致：只有 `/agents` 和 `/vaults` 需要该请求头（[Agents API guide](../api/public-agent-api.md)）。

每个资源还提供 `.with_raw_response` 和 `.with_streaming_response` 包装器，且每个资源类都有 `Async*` 对应实现（[`agents.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/agents.py)、[`sessions.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/sessions.py)）。

### 应用会接触的资源 {#resources-an-application-touches}

| 资源 | SDK 访问方式 | 操作 |
| --- | --- | --- |
| Agents | `client.beta.agents` | create、retrieve、update、list、delete |
| Sessions | `client.beta.agents.sessions` | create（JSON 或 `stream=True`）、retrieve、update、list、delete，以及 `stream` helper |
| Session events | `client.beta.agents.sessions.events` | create（输入）、stream（SSE） |
| Turns | `client.beta.agents.sessions.turns` | retrieve、list |
| Items | `client.beta.agents.sessions.items` | list |
| Subagents | `client.beta.agents.sessions.subagents` | retrieve、list；`subagents.items.list`；`subagents.turns.retrieve`/`list`；`subagents.turns.items.list` |
| Environments | `client.beta.agents.environments` | retrieve |
| Environment files | `client.beta.agents.environments.files` | create（`file_id` 或 `inline`）、list |
| Environment Templates | `client.beta.agents.environments.templates` | create、retrieve、update、list、delete |
| Vaults | `client.beta.agents.vaults` | create、retrieve、list、delete |
| Vault Credentials | `client.beta.agents.vaults.credentials` | create、retrieve、update、list、delete |
| Artifacts | `client.beta.agents.sessions.artifacts` | retrieve、list、delete、content |
| Files | `client.files` | create、retrieve、list、delete、content |
| Skills | `client.skills`、`client.skills.versions` | create、retrieve、update、list、delete、content；versions 的 create、retrieve、list、delete、content |

各资源的形状已对照固定版本的资源文件验证：[agents](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/agents.py)、[sessions](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/sessions.py)、[events](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/events.py)、[turns](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/turns.py)、[items](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/items.py)、[artifacts](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/artifacts.py)、[subagents](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/subagents/subagents.py)、[environments](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/environments/environments.py)、[environment files](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/environments/files.py)、[templates](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/environments/templates.py)、[vaults](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/vaults/vaults.py)、[credentials](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/vaults/credentials.py)、[files](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/files.py)、[skills](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/skills/skills.py) 和 [versions](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/skills/versions/versions.py)。

值得了解的使用要点：

- **Agents** 是可复用配置。`create` 要求 `model`，并接受 `instructions`、`metadata`（最多 16 对）、`name`、`reasoning`、`service_tier`、`text`、`tools` 和 `multi_agent`（[`agent_create_params.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/types/beta/agent_create_params.py)）；`update` 只修改发送的字段，`metadata` 会替换所有键值对（[`agent_update_params.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/types/beta/agent_update_params.py)）。Function 工具声明为 `{"type": "function", "name", "description", "parameters"}`（[`persisted_agent_tool_param.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/types/beta/persisted_agent_tool_param.py)）。
- **Sessions** 创建时要求 `environment`，可选 `agent_id`/内联 `agent`、`input`、`metadata`、`stream` 和 `vault_ids`（[`sessions.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/sessions.py)）。`environment` 是 `{"type": "none"}`、`{"type": "openai_hosted", ...}` 或 `{"type": "self_hosted", "workspace_directory": ...}` 之一（[`environment_param.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/types/beta/environment_param.py)）。使用 `stream=True` 时，POST 响应本身是 `Stream[AgentSessionEvent]`；否则是 `AgentSession`，其 `status` 为 `idle`、`in_progress`、`requires_action`、`failed`（[`agent_session.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/types/beta/agent_session.py)）。`update` 只接受 `metadata`（[`session_update_params.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/types/beta/agents/session_update_params.py)）。
- **Session events**：`events.create(session_id, events=[...], idempotency_key=...)` 不返回内容（服务端返回 202），`events.stream(session_id)` 返回带 `Accept: text/event-stream` 的 `Stream[AgentSessionEvent]`（[`events.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/events.py)）。输入事件恰好有三种变体：`agent.session.input.message`、`agent.session.input.cancel` 和 `agent.session.input.tool_result`（[`agent_session_input_param.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/types/beta/agent_session_input_param.py)）。事件联合体有 30 个成员，包括 session 状态事件、turn 生命周期事件、文本和推理增量、item 事件以及 subagent 事件（[`agent_session_event.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/types/beta/agent_session_event.py)）。
- **Turns 和 Items** 通过 `turns.retrieve`/`list` 和 `items.list` 读取，均以 `after`、`limit` 和 `order` 分页（[`turns.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/turns.py)、[`items.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/items.py)）。Turn 的 `status` 为 `queued`、`in_progress`、`waiting`、`completed`、`failed`、`cancelled` 之一，其 `usage` 可选（[`turn.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/types/beta/agents/sessions/turn.py)、[`token_usage.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/types/beta/token_usage.py)）。
- **Artifacts** 按 session 列出，用 `content(artifact_id, session_id=...)` 下载（返回 SDK 的二进制响应对象），删除时不会触碰底层工作区文件（[`artifacts.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/artifacts.py)）。
- **Subagents** 是只读的子任务：检索和列出 subagent、列出其 Items，并读取其 Turns 及其 Items（[`subagents.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/subagents/subagents.py)）。
- **Vault credentials** 接受 `auth` 对象和 `name`；secret 值只写（[`credentials.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/vaults/credentials.py)）。

### 应用编写的端到端流程 {#the-end-to-end-flow-an-application-writes}

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

分步说明，以及应用必须了解的机制：

1. **创建 Agent。** `model` 必填；`instructions`、`name`、`metadata`、`reasoning`、`service_tier`、`text`、`tools` 和 `multi_agent` 可选（[`agents.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/agents.py)）。
2. **创建 Session。** 固定版本 SDK 对 `create` 做了重载：`stream=True` 类型为 `Stream[AgentSessionEvent]`，默认类型为 `AgentSession`；两者使用相同的 POST 请求体（[`sessions.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/sessions.py)）。内联 `agent`（而不是 `agent_id`）会为该 session 覆盖字段。
3. **打开事件流。** `sessions.events.stream(session_id)` 以 `Stream[AgentSessionEvent]` 返回实时 SSE 流（[`events.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/events.py)）。请在发送输入前打开它；它不会重放错过的事件（[Agents API guide](../api/public-agent-api.md)）。
4. **发送输入事件。** `events.create(session_id, events=[...], idempotency_key=...)` 提交消息、取消或工具结果，并在 202 响应时返回 `None`（[`events.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/events.py)）。三种输入变体是 message、cancel 和 tool result（[`agent_session_input_param.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/types/beta/agent_session_input_param.py)）。
5. **处理 `requires_action` 和函数调用。** 当模型调用函数时，session 报告 `requires_action`，并给出带 `call_id`、`turn_id`、`name` 和 `arguments` 的 `function_call` required action（[`agent_session.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/types/beta/agent_session.py)）。同一调用也会以 `agent.session.turn.item.added` 事件的形式到达流，其 item 是 `function_call`（[`agent_function_call_item.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/types/beta/agent_function_call_item.py)）。用 `agent.session.input.tool_result` 提交结果，`success` 加上 `output`（字符串或输入内容部分列表）或 `error`（[`agent_function_call_output_param.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/types/beta/agent_function_call_output_param.py)）。
6. **使用 `sessions.stream` 自动化循环。** 该 helper 是大多数小型应用使用的便利封装；其确切行为在下一节描述。
7. **读取 Turns 和 Items** 以获得持久历史和恢复能力，并**下载**已完成 turn 产生的 **Artifacts**（[`artifacts.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/artifacts.py)）。

不使用 helper 时，应用手动驱动同一流程：

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

更完整的流程 —— 异步用法、流式创建与取消、文件与 Artifacts、Skills 与模板、Vaults 与 MCP、自托管 Session 以及类型化错误 —— 见 [Agents API guide](../api/public-agent-api.md#worked-examples) 和可运行的 [`example/hosted-agents-python`](../../../example/hosted-agents-python) 脚本。

### SDK 便利功能与注意事项 {#sdk-conveniences-and-gotchas}

**分页。** 列表方法返回 `SyncCursorPage` / `AsyncPaginator`（异步）；页对象暴露 `.data` 和 `.has_more`，迭代时自动分页，并使用最后一项的 ID 作为下一个 `after` 游标（[`pagination.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/pagination.py)、[`turns.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/turns.py)）。有一个列表例外：`environments.files.list` 返回 `SyncTokenPage[EnvironmentFile]`，使用不透明的 `.next` token 分页，而不是 `after`（[`files.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/environments/files.py)、[`pagination.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/pagination.py)）。

**Idempotency-Key。** 只有两个调用点接受命名参数 `idempotency_key`：`sessions.events.create` 和 `sessions.stream` helper（[`events.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/events.py)、[`sessions.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/sessions.py)）。`sessions.create` 没有该参数；应用通过 `extra_headers={"Idempotency-Key": ...}` 传入，OpenAgentCore 指南也是这样展示的（[Agents API guide](../api/public-agent-api.md)）。helper 在未提供时为主输入生成 `uuid4` 键，并为每个工具结果生成新键；它会从共享请求选项中剥离任何 `Idempotency-Key`，因此输入键绝不会被复用于工具结果（[`_streams.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/lib/streaming/agents/_streams.py)）。它会以 0.1 秒、0.3 秒和 0.6 秒重试已知的 "Unknown pending tool call" 注册竞态（[`_streams.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/lib/streaming/agents/_streams.py)，竞态判定在 [`_tools.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/lib/streaming/agents/_tools.py)）。

**`sessions.stream` helper 详解。** 它要求 session 处于 idle，否则抛出 `ValueError`；它在提交输入*之前*订阅事件流；只接受单一写入者；handler 按函数名查找并接收解析后的参数字典；已注册的 handler 在 call 事件被 yield 后顺序运行；未注册的工具留给调用方；handler 异常会提交 `success: false` 和通用消息 "Tool handler failed."，绝不包含异常文本；迭代在 `agent.session.failed` 或 turn 结束后的 `agent.session.idle` 处停止；流提前结束会抛出 `RuntimeError`；`close()` 不会取消后端 turn（[`_streams.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/lib/streaming/agents/_streams.py)）。

**`extra_body`。** 每个生成的方法都接受 `extra_body`，SDK 会将其最后合并进 JSON 请求体。这是发送 OpenAgentCore `x_agents_core` 字段的受支持方式，Core 指南也有展示（[Agents API guide](../api/public-agent-api.md)）；固定版本 SDK 本身没有 `x_agents_core` 参数。

**流式上下文管理器。** `events.stream` 和 `sessions.stream` 都以 `with ... as stream:`（或 `async with`）使用；返回的 `Stream`/`AsyncStream` 支持迭代和 `close()`（[`events.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/events.py)、[`_streams.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/lib/streaming/agents/_streams.py)）。

**同步/异步对等。** 每个资源都提供参数相同的 `Agents`/`AsyncAgents`、`Sessions`/`AsyncSessions` 等；异步 stream helper 同时接受普通和可等待的 handler 结果（[`sessions.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/beta/agents/sessions/sessions.py)、[`_streams.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/lib/streaming/agents/_streams.py)）。

**客户端侧与服务端侧。** 服务端运行 agent、执行 harness 并拥有 Turns、Items 和 Environments。SDK 是传输层加上 `sessions.stream` 工具调用循环；它自身从不执行工具，也不在该 helper 之外运行 agent 循环。helper 的 "循环" 只是 subscribe → yield → submit tool results → stop at terminal events。

**重试。** SDK 对连接错误和 408/409/429/5xx 响应进行重试，最多 `max_retries`（默认 2）次，采用指数退避（[`_constants.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/_constants.py)）；重试的 `events.create` 调用只有在调用方提供了 `Idempotency-Key` 时才会复用它，这也是 helper 生成键的原因。

### 版本说明：固定版本 3.13.0 与当前版本 3.24.0 {#version-notes-pin-3-13-0-vs-current-3-24-0}

固定版本是 3.13.0，发布于 2026-09-10；撰写时最新版本是 3.24.0，发布于 2026-10-02（[PyPI openai](https://pypi.org/project/openai/)）。本节所有内容都**晚于固定版本**，不得用于固定基线；OpenAgentCore 的契约测试会拒绝固定版本之外的操作和字段（[coverage ledger](../../../contracts/agents-api/zh/index.md)）。

- `sessions.traces.list(session_id)` 是新资源，以 OTLP JSON 列出已发布的根 turn traces，按 turn 创建时间排序，支持 `after`/`limit`/`order`（[`traces.py`](https://github.com/openai/openai-python/blob/v3.24.0/src/openai/resources/beta/agents/sessions/traces.py)）。
- `sessions.create` 新增 `output_type: type[OutputT] | None` 参数（[`sessions.py`](https://github.com/openai/openai-python/blob/v3.24.0/src/openai/resources/beta/agents/sessions/sessions.py)）。
- Session 输入新增 `agent.session.input.computer_use_approval_request_result`，带浏览器认证的 cancel/submit 和 origin 访问响应（[`agent_session_input_param.py`](https://github.com/openai/openai-python/blob/v3.24.0/src/openai/types/beta/agent_session_input_param.py)）。
- 新增 `agent.session.environment.reset` 事件，报告已替换的托管沙箱；对话历史保留，但文件和进程不保留（[`agent_session_environment_reset_event.py`](https://github.com/openai/openai-python/blob/v3.24.0/src/openai/types/beta/agent_session_environment_reset_event.py)）。
- 新增 `artifacts.for_result(...)` 以及环境文件的 `prepare`/`prepare_directory`/`upload` helper（[`artifacts.py`](https://github.com/openai/openai-python/blob/v3.24.0/src/openai/resources/beta/agents/sessions/artifacts.py)、[`files.py`](https://github.com/openai/openai-python/blob/v3.24.0/src/openai/resources/beta/agents/environments/files.py)）。

我比较了固定版本与 3.24.0 wheel 的资源文件列表和方法名，未发现固定版本操作被移除；当前版本在 `beta/agents` 上看起来是超集。我没有逐字段比对所有类型，因此其他位置可能存在新增的可选字段。

## 第二部分 —— openai-agents-python，客户端 Agents SDK {#part-b-—-openai-agents-python-the-client-side-agents-sdk}

### 安装与 hello world {#installation-and-hello-world}

```sh
pip install openai-agents
```

该包要求 Python 3.10 或更高版本（[README](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/README.md)），并依赖 `openai>=3.0.0,<4`（[pyproject.toml](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/pyproject.toml)）。

```python
# openai-agents 0.23.1
from agents import Agent, Runner

agent = Agent(name="Assistant", instructions="You are a helpful assistant")

result = Runner.run_sync(agent, "Write a haiku about recursion in programming.")
print(result.final_output)
```

`Runner.run` 是异步入口，`Runner.run_sync` 为同步调用方封装它（不能在运行中的事件循环内使用），`Runner.run_streamed` 返回 `RunResultStreaming`，其 `stream_events()` 产生语义化流事件（[`run.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/run.py)、[`result.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/result.py)）。`RunResult` 暴露 `final_output`、`new_items`、`raw_responses`、`last_agent` 和 `to_input_list()`，供后续运行使用（[`result.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/result.py)）。上面的 hello-world 正是文档中的第一个示例（[docs/index.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/index.md)、[quickstart](https://openai.github.io/openai-agents-python/quickstart/)）。

### 它调用哪个 OpenAI API {#which-openai-api-it-calls}

**已对照当前版本验证：** 该框架不调用托管 Agents API。其默认 OpenAI 模型 `OpenAIResponsesModel` 对非流式请求调用 `client.responses.create(...)`，对流式请求调用 `client.responses.with_streaming_response.create(...)`（[`openai_responses.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/models/openai_responses.py)）。在 `src/` 中搜索 `beta.agents`、`beta/agents` 或 `agents=v1` 没有任何结果。替代方案 `OpenAIChatCompletionsModel` 调用 `client.chat.completions.create(...)`（[`openai_chatcompletions.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/models/openai_chatcompletions.py)）。

因此，该框架可以指向实现了 Responses API（或 Chat Completions）的服务器，但不能指向只实现托管 Agents API 的服务器。要从该框架驱动托管 Agents API，需要自定义 `Model` 实现；`Model` 接口是抽象的（`get_response`、`stream_response`），且 SDK 没有附带 `beta.agents` 适配器（[`interface.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/models/interface.py)）。

默认 provider 是 `MultiProvider`，它把无前缀的模型名（或 `openai/` 前缀）路由到 `OpenAIProvider`，并可将 `litellm/` 和 `any-llm/` 前缀路由到别处（[`multi_provider.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/models/multi_provider.py)、[`run_config.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/run_config.py)）。除非 `use_responses=False`，`OpenAIProvider` 会把模型名解析为 `OpenAIResponsesModel`（[`openai_provider.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/models/openai_provider.py)）。

配置接口，均**已对照当前版本验证**：

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

- `set_default_openai_key(key, use_for_tracing=True)` 和 `set_default_openai_client(client, use_for_tracing=True)` 是公开包装器；第二个参数控制该 key/client 是否也用于 trace 导出（[`__init__.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/__init__.py)、[`_config.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/_config.py)）。
- provider 自行构造客户端时，会传入 SDK 级默认 key（如果设置过）和 `os.getenv("OPENAI_BASE_URL")`；未设置 key 时 openai SDK 会从环境填充 `OPENAI_API_KEY`，而 `OPENAI_WEBSOCKET_BASE_URL` 用于 Responses-over-WebSocket（[`openai_provider.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/models/openai_provider.py)、[docs/config.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/config.md)）。
- 向 `OpenAIProvider` 传入 `openai_client=` 时不能与 `api_key`、`base_url`、`websocket_base_url`、`organization` 或 `project` 组合；否则 provider 抛出 `UserError`（[`openai_provider.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/models/openai_provider.py)）。
- `RunConfig(model_provider=...)` 为单次运行覆盖 provider，`RunConfig(model=...)` 设置默认模型（[`run_config.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/run_config.py)）。

### 应用使用的构建块 {#the-building-blocks-applications-use}

**Agent。** `Agent` 是 dataclass，字段包括 `name`、`instructions`（字符串或返回字符串的可调用对象）、`model`（字符串或 `Model`）、`tools`、`mcp_servers`、`handoffs`、`input_guardrails`、`output_guardrails`、`output_type`、`model_settings`、`hooks` 和 `tool_use_behavior`（[`agent.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/agent.py)）。省略 `model` 时 SDK 使用 `get_default_model()`，在 0.23.1 中是 `OPENAI_DEFAULT_MODEL` 或 `gpt-5.6-luna`，并采用 GPT-5 默认设置（[`default_models.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/models/default_models.py)、[docs/models/index.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/models/index.md)）。

**Function 工具。** `@function_tool` 包装 Python 函数；schema 由签名、docstring 和类型提示生成，并使用 Pydantic 校验（[`function_schema.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/function_schema.py)、[docs/tools.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/tools.md)）。`tool` 是 `function_tool` 的公开别名（[`decorators.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/decorators.py)）。该装饰器还接受 `name_override`、`description_override`、`strict_mode`、`is_enabled`、`needs_approval`、超时和工具 guardrails（[`tool.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/tool.py)）。

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

**托管工具。** SDK 提供 `WebSearchTool`、`FileSearchTool`、`ComputerTool`、`CodeInterpreterTool`、`ImageGenerationTool`、`HostedMCPTool`、`ToolSearchTool` 和 `ProgrammaticToolCallingTool`（[`tool.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/tool.py)、[docs/tools.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/tools.md)）。文档说明托管工具只能通过 Responses API 与 OpenAI 模型配合使用（[docs/tools.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/tools.md)）；`HostedMCPTool` 是由 Responses API 直接调用的远程 MCP 服务器，而本地 MCP 服务器通过 `Agent(mcp_servers=[...])` 使用 `MCPServerStdio`、`MCPServerSse` 或 `MCPServerStreamableHttp` 附加（[docs/mcp.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/mcp.md)、[`mcp/server.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/mcp/server.py)）。

**Handoffs。** 在 `Agent.handoffs` 中传入 agent，或用 `handoff()` 包装它们，会把每个目标变成一个工具（默认名 `transfer_to_<agent_name>`）；`on_handoff`、`input_type`、`input_filter` 和 `is_enabled` 可定制 handoff（[`handoffs/__init__.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/handoffs/__init__.py)、[docs/handoffs.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/handoffs.md)）。另一种编排模式是 `Agent.as_tool()`（[`agent.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/agent.py)、[docs/multi_agent.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/multi_agent.md)）。

**Guardrails。** `@input_guardrail` 和 `@output_guardrail` 装饰返回 `GuardrailFunctionOutput(tripwire_triggered=...)` 的函数；输入 guardrails 只对第一个 agent 运行，输出 guardrails 只对产生最终输出的 agent 运行。Tripwire 会抛出 `InputGuardrailTripwireTriggered` / `OutputGuardrailTripwireTriggered`（[`guardrail.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/guardrail.py)、[docs/guardrails.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/guardrails.md)）。

**Sessions 与 memory。** 向 `Runner.run` 传入 `session=` 会让 SDK 在运行前加载历史，并在运行后持久化新 item；`SQLiteSession("conversation_123")` 默认在内存中，传入路径时使用文件存储（[`sqlite_session.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/memory/sqlite_session.py)、[docs/sessions/index.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/sessions/index.md)）。extensions 包新增了 SQLAlchemy、Redis、MongoDB、Dapr 和加密 session 后端（[`extensions/memory`](https://github.com/openai/openai-agents-python/tree/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/extensions/memory)）。

**Run context。** `Runner.run(..., context=obj)` 会把 `obj` 作为 `RunContextWrapper[T].context` 传入工具、hooks、guardrails 和 handoff 回调；该包装器还暴露运行的 `usage`（[`run_context.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/run_context.py)、[docs/context.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/context.md)）。

**结构化输出。** `Agent(output_type=SomeModel)` 让 `result.final_output` 成为该类型的实例；默认输出类型是 `str`（[`agent.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/agent.py)、[docs/results.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/results.md)）。

**模型设置与 provider。** `ModelSettings` 可调整 temperature、top_p、reasoning effort/verbosity 等；GPT-5 模型按模型名获得 SDK 默认值（[`model_settings.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/model_settings.py)、[`default_models.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/models/default_models.py)）。非 OpenAI provider 通过 `LitellmModel` / `LitellmProvider` 和 `AnyLLMModel` 适配器或自定义 `ModelProvider` 接入（[`extensions/models`](https://github.com/openai/openai-agents-python/tree/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/extensions/models)、[docs/models/index.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/models/index.md)）。

**Tracing。** Tracing 默认开启。应用可以用环境变量 `OPENAI_AGENTS_DISABLE_TRACING=1` 或 `set_tracing_disabled(True)` 全局禁用它，也可以用 `RunConfig(tracing_disabled=True)` 按运行禁用；用 `set_trace_processors([...])` / `add_trace_processor(...)` 替换或添加 exporter，并用 `set_tracing_export_api_key(...)` 设置导出 key（[`tracing/__init__.py`](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/tracing/__init__.py)、[docs/tracing.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/tracing.md)）。默认 exporter 会把 trace 发送到 OpenAI 平台，因此自托管部署必须禁用 tracing 或安装自定义 processor。

**版本说明。** 以上所有框架结论均对照 0.23.1（仓库 commit `81f0ccf`，发布于 2026-10-02）验证。更早版本有不同的默认值和接口面（例如，该包的 `openai>=3.0.0,<4` 下限在 [docs/config.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/config.md) 中记载为 0.21.0 的变更）；我没有验证 0.23.1 之前的行为。

## 第三部分 —— 两者的关系 {#part-c-—-how-the-two-relate}

| | 托管 Agents API（`client.beta.agents`） | Agents SDK 框架（`openai-agents`） |
| --- | --- | --- |
| 是什么 | OpenAI 托管 Agents 服务的 SDK | 在应用中构建 agent 的开源框架 |
| 调用的 API | `/v1/agents/*`、`/v1/vaults/*`、`/v1/files`、`/v1/skills` | `/v1/responses`（默认）或 `/v1/chat/completions` |
| 谁运行循环 | 服务；客户端订阅并提交工具结果 | 你的进程；`Runner` 循环直到最终输出 |
| 状态 | 服务端 Sessions、Turns、Items、Environments | 客户端历史：`to_input_list()`、`session=`，或 OpenAI 托管的 `previous_response_id` / `conversation_id` |
| 工具 | 在 Agent 上声明；harness 执行它们，你的代码返回结果 | Python 函数在你的进程中运行；托管工具在 Responses API 中运行 |
| 典型客户端 | `openai` 3.13.0（固定）、`client.beta.agents` | `openai-agents` 0.23.1 |

这是两个不同的产品，只是名称里都有 "Agent"。托管 Agents API 是服务端产品：你保存 Agent、启动 Session，并通过事件流与托管执行环境交互（[Agents API guide](../api/public-agent-api.md)）。Agents SDK 框架是客户端库：你在代码中构造 `Agent` 对象，`Runner` 调用 Responses API，并在循环中调度你的 Python 工具（[docs/index.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/index.md)）。该框架自己的文档将这一界限表述为 "Agents SDK or Responses API?"，并指出 SDK 把模型调用包装在更高层的运行时中（[docs/index.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/index.md)）。

**OpenAgentCore 的 `/v1` 对应托管 Agents API。** [coverage ledger](../../../contracts/agents-api/zh/index.md) 指出 Core 的目标是完整的 OpenAI Agents API，固定到 `openai-python` 3.13.0，并精确提供其 58 条路由，Core 专有内容位于 `x_agents_core`（[docs/api/index.md](../api/index.md)）。因此，与 `/v1` 通信的应用应使用 `client.beta.agents`（以及 `client.files` / `client.skills`），而不是该框架。

实际含义如下：

- 基于固定版本 `client.beta.agents` 构建的应用可在 `/v1` 上使用 58 条固定路由，但受 Core 已记录的差异约束（[coverage ledger](../../../contracts/agents-api/zh/index.md)）。SDK 会自动设置 Beta 请求头；纯 HTTP 客户端必须在 `/agents` 和 `/vaults` 上设置 `OpenAI-Beta: agents=v1`（[Agents API guide](../api/public-agent-api.md)）。
- 基于 `openai-agents` 构建的应用不能简单地把 `OPENAI_BASE_URL` 指向 `/v1`。该框架调用 `/responses`，而 Core 不提供该端点；`/v1` 只有固定的 Agents、Files 和 Skills 路由（[upstream-routes.json](../../../contracts/agents-api/upstream-routes.json)）。托管工具和默认 tracing 也指向 OpenAI 服务，而不是自托管的 Core。
- 理论上可以自定义桥接：应用可以在 `client.beta.agents` 之上实现 `agents.models.interface.Model`，让框架的 `Runner` 提交 session 事件而不是 Responses 请求；但 SDK 没有附带这样的适配器，而且工具调用循环、guardrails 和 handoffs 将因此在框架与托管 harness 之间分裂。OpenAgentCore 和该框架目前都没有这样做。
- Core 的 `x_agents_core.model_provider` `responses` 协议与此选择无关。它命名的是 Harness 调用模型端点所用的协议（Codex 的原生协议是 `responses`）（[model execution](../../../contracts/agents-api/zh/model-execution.md)）；它并不会让 Core 的 `/v1` 变成 Responses API。

## 这对 OpenAgentCore 意味着什么 {#what-this-means-for-openagentcore}

- **受支持的客户端。** 应用应安装 `openai==3.13.0` 并使用 `client.beta.agents`、`client.files` 和 `client.skills`（[upstream.json](../../../contracts/agents-api/upstream.json)、[Agents API guide](../api/public-agent-api.md)）。Core 的契约测试将路由器和 schema 固定到该版本，更新的 SDK 操作或字段需等待协议升级（[coverage ledger](../../../contracts/agents-api/zh/index.md)）。
- **请求头。** Core 要求 `/agents` 和 `/vaults` 上的 `OpenAI-Beta: agents=v1`；固定版本 SDK 会按请求添加它，而 Files 和 Skills 路由不需要 Beta 请求头（[Agents API guide](../api/public-agent-api.md)、[`files.py`](https://github.com/openai/openai-python/blob/d7c41efee1b0802b79f3f88a678ef2052b06e9ce/src/openai/resources/files.py)）。
- **Core 扩展。** `harness`、`model_provider`、`harness_config`、`environment` 和 `installation` 通过 SDK 的 `extra_body` 在 `x_agents_core` 中传递；Core 拒绝任何其他成员（[Agents API guide](../api/public-agent-api.md)）。
- **工具循环。** 固定版本 SDK 的 `sessions.stream(..., tool_handlers=...)` helper 是函数工具的预期便利封装，与 Core 的模型一致：先订阅、提交输入、返回结果、在 `idle` 或 `failed` 处停止。helper 的前置条件（idle session、单一写入者）由应用负责。
- **幂等性。** 当创建重试复用同一个 `Idempotency-Key` 时，Core 返回原始 Session；官方服务会创建新 Session，而 Core 的 credential 创建完全不接受键（[coverage ledger](../../../contracts/agents-api/zh/index.md)）。应用必须通过 `extra_headers` 在 `sessions.create` 上发送该键，因为固定版本 SDK 在那里没有命名参数。
- **流。** 事件流是仅实时的；应用从 Turns 和 Items 恢复缺口（[Agents API guide](../api/public-agent-api.md)）。SDK helper 有意不重放错过的事件，与 Core 一致。
- **框架预期。** 如果用户问 `openai-agents` 能否驱动 OpenAgentCore，准确的答案是不能：它调用 Responses API，而 `/v1` 实现的是托管 Agents API。最接近的受支持体验是 `client.beta.agents` 加 `sessions.stream` 工具 handler 循环；该框架的 handoffs、guardrails 和客户端 sessions 与 `/v1` 功能并非一一对应（Core 的 Subagents 是服务端子任务，不是框架的 `handoff()` 原语）。

## 来源 {#sources}

固定 SDK —— openai-python 3.13.0，commit `d7c41efee1b0802b79f3f88a678ef2052b06e9ce`（<https://github.com/openai/openai-python/tree/d7c41efee1b0802b79f3f88a678ef2052b06e9ce>）：

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

当前 SDK —— openai-python 3.24.0，tag `v3.24.0`（commit `637f1b8b2e9fdc3220fd4edbb8602cc89dc489c8`），仅用于 "晚于固定版本" 的说明：

- [src/openai/resources/beta/agents/sessions/traces.py](https://github.com/openai/openai-python/blob/v3.24.0/src/openai/resources/beta/agents/sessions/traces.py), [sessions.py](https://github.com/openai/openai-python/blob/v3.24.0/src/openai/resources/beta/agents/sessions/sessions.py), [environments/files.py](https://github.com/openai/openai-python/blob/v3.24.0/src/openai/resources/beta/agents/environments/files.py), [sessions/artifacts.py](https://github.com/openai/openai-python/blob/v3.24.0/src/openai/resources/beta/agents/sessions/artifacts.py)
- [src/openai/types/beta/agent_session_input_param.py](https://github.com/openai/openai-python/blob/v3.24.0/src/openai/types/beta/agent_session_input_param.py), [agent_session_environment_reset_event.py](https://github.com/openai/openai-python/blob/v3.24.0/src/openai/types/beta/agent_session_environment_reset_event.py)
- PyPI: <https://pypi.org/project/openai/3.24.0/>; release data read from the [PyPI JSON API](https://pypi.org/pypi/openai/json)

Agents SDK —— openai-agents-python 0.23.1，commit `81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3`：

- [pyproject.toml](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/pyproject.toml), [src/agents/version.py](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/version.py), [README.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/README.md)
- [src/agents/agent.py](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/agent.py), [run.py](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/run.py), [result.py](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/result.py), [run_config.py](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/run_config.py), [run_context.py](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/run_context.py), [__init__.py](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/__init__.py), [_config.py](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/_config.py)
- [src/agents/models/openai_responses.py](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/models/openai_responses.py), [openai_chatcompletions.py](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/models/openai_chatcompletions.py), [openai_provider.py](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/models/openai_provider.py), [multi_provider.py](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/models/multi_provider.py), [default_models.py](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/models/default_models.py), [interface.py](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/models/interface.py)
- [src/agents/tool.py](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/tool.py), [decorators.py](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/decorators.py), [function_schema.py](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/function_schema.py), [guardrail.py](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/guardrail.py), [handoffs/__init__.py](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/handoffs/__init__.py), [memory/sqlite_session.py](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/memory/sqlite_session.py), [mcp/server.py](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/mcp/server.py), [tracing/__init__.py](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/tracing/__init__.py), [extensions/models](https://github.com/openai/openai-agents-python/tree/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/extensions/models), [extensions/memory](https://github.com/openai/openai-agents-python/tree/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/src/agents/extensions/memory)
- Docs: [index.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/index.md), [quickstart.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/quickstart.md), [config.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/config.md), [tools.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/tools.md), [mcp.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/mcp.md), [sessions/index.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/sessions/index.md), [tracing.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/tracing.md), [guardrails.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/guardrails.md), [models/index.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/models/index.md), [context.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/context.md), [multi_agent.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/multi_agent.md), [handoffs.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/handoffs.md), [results.md](https://github.com/openai/openai-agents-python/blob/81f0ccf20c6e24063b9da36fa37f2bdb6a43d8d3/docs/results.md)
- Documentation site: <https://openai.github.io/openai-agents-python/>
- PyPI: <https://pypi.org/project/openai-agents/0.23.1/>; release data read from the [PyPI JSON API](https://pypi.org/pypi/openai-agents/json)

OpenAgentCore（本仓库）：

- [contracts/agents-api/upstream.json](../../../contracts/agents-api/upstream.json)、[upstream-routes.json](../../../contracts/agents-api/upstream-routes.json)、[coverage ledger](../../../contracts/agents-api/zh/index.md)
- [docs/api/public-agent-api.md](../api/public-agent-api.md)、[docs/api/index.md](../api/index.md)
- [contracts/agents-api/model-execution.md](../../../contracts/agents-api/zh/model-execution.md)
