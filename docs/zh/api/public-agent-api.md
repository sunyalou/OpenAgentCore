---
title: "Agents API 指南"
source: docs/api/public-agent-api.md
source_hash: 66b2daebd5e8b55fe04c319e1fbd833398099e5c14b1bca8fe5121dd64ddcf3c
---

Core 在 `/v1` 提供 [OpenAI Agents API](https://platform.openai.com/docs/api-reference)。可以使用官方 OpenAI SDK 或普通 HTTP。本指南针对每项常见操作同时展示这两种方式，并说明 Core 与 OpenAI 存在差异的地方。

初次使用此 API？请先运行[快速入门](../getting-started/quickstart.md)。

## 准备开始 {#before-you-start}

**基础 URL 和密钥。** 管理员会为你提供 API 基础 URL，例如 `https://core.example/v1`，以及 Project API 密钥。

```sh
export OPENAI_BASE_URL=https://core.example/v1
read -rs OPENAI_API_KEY && export OPENAI_API_KEY
```

**SDK。** 使用固定版本：

```sh
pip install openai==3.13.0
```

```python
from openai import OpenAI

client = OpenAI()  # reads OPENAI_BASE_URL and OPENAI_API_KEY
```

**HTTP。** 每个请求都需要 Bearer 密钥。`/agents` 和 `/vaults` 下的路由还需要 `OpenAI-Beta: agents=v1`；`/files` 和 `/skills` 则不需要。SDK 会同时设置这两者。下面的 HTTP 示例使用以下 shell 辅助函数：

```sh
oac() {  # oac PATH [curl options]: call /agents or /vaults with the required headers
  curl -sS "$OPENAI_BASE_URL$1" \
    -H "Authorization: Bearer $OPENAI_API_KEY" \
    -H "OpenAI-Beta: agents=v1" \
    -H "Content-Type: application/json" "${@:2}"
}
oac /agents
```

在共享主机上，`-H @<(printf 'Authorization: Bearer %s\n' "$OPENAI_API_KEY")` 可以避免密钥出现在进程列表中。

## 资源概览 {#resources-at-a-glance}

| 资源 | 路径 | 用途 |
| --- | --- | --- |
| [Agents](#agents) | `/agents` | 可复用配置：模型、指令、工具和 harness |
| [Sessions](#sessions) | `/agents/sessions` | 一次 Agent 对话及其独立的 Environment |
| [输入事件](#send-input) | `/agents/sessions/{id}/events`（POST） | 消息、取消和工具结果 |
| [事件流](#stream-events) | `/agents/sessions/{id}/events`（GET） | 实时的服务器发送事件 |
| [Turns 和 Items](#turns-and-items) | `/agents/sessions/{id}/turns`、`/items` | 持久化历史 |
| [Files](#files) | `/files`、`/agents/environments/{id}/files`、`/agents/sessions/{id}/artifacts` | 上传内容、工作区文件和输出 |
| [Skills](#skills) | `/skills` | 带版本的能力捆绑包 |
| [Environment Templates](#environment-templates) | `/agents/environments/templates` | 可复用的工作区设置 |
| [Vaults](#vaults) | `/vaults` | MCP 服务器的只写凭据 |
| Subagents | `/agents/sessions/{id}/subagents` | 只读的子任务；请参阅 [subagents](../../../contracts/agents-api/zh/subagents.md) |

Core 的路由与固定版本 SDK 中的路由完全一致，完整列表见 [upstream-routes.json](https://github.com/MiniMax-AI/OpenAgentCore/blob/main/contracts/agents-api/upstream-routes.json)。Core 不添加任何路由；其扩展内容位于 [`x_agents_core`](#core-extensions-x-agents-core)。

## 常见任务 {#common-tasks}

| 目标 | 使用方式 |
| --- | --- |
| 继续对话或引导正在运行的 Turn | 向同一 Session [发送消息](#send-a-message)。如果需要使用不同配置或工作区，请创建新的 Session |
| 实时查看输出 | [流式传输事件](#stream-events) |
| 停止当前 Turn，或在响应丢失后恢复 | [取消](#cancel)、[幂等性](#idempotency) |
| 从 Agent 调用你自己的代码 | [函数工具](#function-tools) |
| 为 Agent 提供 Skills、软件包、文件和设置命令 | [Skills](#skills)、[Environment Templates](#environment-templates) |
| 连接需要凭据的 MCP 服务器 | [Vaults](#vaults) 和 [execution tools](../../../contracts/agents-api/zh/execution-tools.md) |
| 在自己的计算机上使用 Skill 或 Plugin 目录 | [本地能力目录](../getting-started/self-hosted.md#local-capability-directories) |
| 将文件放入工作区，或下载 Agent 生成的内容 | [Files](#files) |
| 读取对话和工具结果 | [Turns 和 Items](#turns-and-items) |
| 查明 Session 或 Turn 失败的原因 | [诊断故障](#diagnose-a-failure) |

## 约定 {#conventions}

### 分页 {#pagination}

列表返回：

```json
{"object": "list", "data": [...], "has_more": true, "first_id": "...", "last_id": "..."}
```

| 参数 | 含义 |
| --- | --- |
| `limit` | 1–100，默认值为 20 |
| `order` | `desc`（默认值）或 `asc` |
| `after` | 传入上一页的 `last_id` |

SDK 会自动为你分页：

```python
for session in client.beta.agents.sessions.list(limit=100):
    print(session.id, session.status)
```

```sh
oac "/agents/sessions?limit=100&after=$LAST_ID"
```

有两个列表存在差异：`GET /files` 一次最多返回 10,000 个文件；工作区文件使用不透明的 `page` 令牌（请参阅[工作区文件](#workspace-files)）。

### 幂等性 {#idempotency}

创建 Session 或发送输入时，请发送 `Idempotency-Key`（最长 128 字节）。使用相同密钥和正文重试时，会返回原始结果，而不会重复执行工作。相同密钥搭配不同正文会失败，并返回 409 `idempotency_conflict`。

```python
import uuid

key = str(uuid.uuid4())  # store it before sending, reuse it on retry
client.beta.agents.sessions.events.create(session_id, events=[...], idempotency_key=key)
client.beta.agents.sessions.create(environment=..., extra_headers={"Idempotency-Key": key})
```

**响应丢失后，**请使用相同密钥重试，然后读取 Session、Turns 和 Items。绝不要在没有密钥的情况下重新发送。即使 Session 已删除，Core 仍会保留创建时使用的密钥。

有关比较规则及其与 OpenAI 的差异，请参阅[创建重试](../../../contracts/agents-api/zh/wire-semantics.md#creation-retries)。

### 错误 {#errors}

```json
{"error": {"type": "invalid_request_error", "message": "...", "code": "model_provider_required", "param": "x_agents_core.model_provider"}}
```

| 状态 | 常见代码 | 含义 |
| --- | --- | --- |
| 400 | `invalid_request_error`、`invalid_beta`、`model_provider_required`、`unsupported_or_invalid_configuration` | 修正请求。`invalid_beta` 表示 `OpenAI-Beta` 请求头缺失或错误 |
| 401 | `invalid_api_key` 或无代码 | 密钥错误或缺失，或者使用了其他命名空间中的密钥 |
| 404 | Beta 路由返回 `not_found_error` | 对象不存在，或属于其他 Project。这两种情况看起来完全相同 |
| 405 | `unsupported_operation` | Core 不支持此操作 |
| 409 | `conflict_error`、`idempotency_conflict` | 状态冲突，例如删除正在繁忙运行的 Session |
| 413 | `request_too_large` | 正文过大 |
| 503 | `authentication_unavailable`、`execution_unavailable` | 暂时无法确定状态；重试前应先读取状态 |

每个响应都包含 `X-Request-Id`；报告问题时请包含该值。

### Core 扩展：`x_agents_core` {#core-extensions-x-agents-core}

Core 可以运行多种 harness，并允许接入你自己的模型访问方式。这些设置是对 OpenAI 数据结构仅有的扩展，位于 `x_agents_core` 内：

| 字段 | 所在位置 | 值 |
| --- | --- | --- |
| `harness` | Agent，或 Session 的内联 `agent` | `codex`、`claude_sdk` 或 `mcode` |
| `model_provider` | Agent，或 Session 创建请求（顶层） | `protocol`、`base_url`、`api_key`；对于 `mcode`，还包括 `context_window` 和 `max_output_tokens`。协议必须是该 harness 的原生协议之一 |
| `harness_config` | Agent、内联 `agent`，或 Session 创建请求（顶层，优先） | harness 的原生模型参数，例如 Codex 的 `model_reasoning_effort` |
| `environment` | `openai_hosted` 或 `self_hosted` Session 创建请求（顶层） | 可移植的准备配置：`environment_template_id`、`files`、`env`、`packages`、`setup_commands`、`skills`、`plugins`、`capability_directories`。同一字段不能也出现在 `environment` 中；请参阅 [Environments](../../../contracts/agents-api/zh/environments.md#preparation-order) |
| `installation` | 只读，适用于 `self_hosted` Session | 在你的计算机上运行的短期安装命令；请参阅 [self-hosted execution](../getting-started/self-hosted.md) |

任何其他成员都会导致 400 错误。`api_key` 为只写字段；读取时会返回 `api_key_configured`。`harness_config` 会替换整个对象；传入 `{}` 可将其清除。使用 SDK 时，请通过 `extra_body` 传入这些字段。

## 选择 harness 和模型 {#choose-a-harness-and-a-model}

harness 是运行 Session 的 Agent 程序：Codex（`codex`）、Claude Code（`claude_sdk`）或 MiniMax Code（`mcode`）。请在 Agent 或内联 `agent` 上设置 `x_agents_core.harness`；如果未设置，则使用安装的默认 harness（[`core.default_harness`](../configuration.md#settings)，除非操作员另行更改，否则为 Codex）。

- **模型。** `model` 是提供商给出的准确模型 ID。`openai_hosted` 或 `none` Session 上的内联 Agent 可以省略此字段，以使用其 harness 的默认模型配置。保存的 Agent 始终必须指定模型。
- **提供商。** harness 会使用其原生协议之一直接调用你的提供商；系统不会进行转换，不匹配时会在创建 Session 阶段拒绝请求。[Model execution](../../../contracts/agents-api/zh/model-execution.md#saved-defaults-and-precedence) 列出了每个 harness 的协议，以及各类 Environment 上 Session 使用的提供商。Session 会在创建时冻结其提供商。
- **原生参数。** `harness_config` 承载 harness 自身的模型设置；请参阅[原生模型参数](../../../contracts/agents-api/zh/model-execution.md#native-model-parameters)。

并非每种 harness、部署位置和操作组合都受支持；[Harness capabilities](../../../contracts/agents-api/zh/harness-capabilities.md) 列出了受支持的组合。

## Agents（智能体） {#agents}

Agent 是保存的配置。Session 启动时会复制该配置，因此编辑 Agent 只会影响新的 Session。

```python
agent = client.beta.agents.create(
    model="your-model-id",
    name="Reviewer",
    instructions="Review code changes and report problems.",
    extra_body={"x_agents_core": {"harness": "codex"}},
)
```

```sh
oac "/agents" -d '{
  "model": "your-model-id",
  "name": "Reviewer",
  "instructions": "Review code changes and report problems.",
  "x_agents_core": {"harness": "codex"}
}'
```

```json
{"id": "00000000-0000-4000-8000-000000000001", "object": "agent", "model": "your-model-id", "name": "Reviewer",
 "instructions": "...", "tools": [], "metadata": {}, "created_at": 1790000000,
 "x_agents_core": {"harness": "codex"}}
```

| 操作 | SDK | HTTP |
| --- | --- | --- |
| 创建 | `agents.create(...)` | `POST /agents` |
| 读取 | `agents.retrieve(id)` | `GET /agents/{id}` |
| 更新 | `agents.update(id, ...)` | `POST /agents/{id}` |
| 列出 | `agents.list()` | `GET /agents` |
| 删除 | `agents.delete(id)` | `DELETE /agents/{id}` |

（全文中的 `agents` 均指 `client.beta.agents`。）

- **更新**只会更改你发送的字段。`metadata` 会替换所有键值对；`null` 会清除 `name` 或 `instructions`。
- **删除**不会影响现有 Session。
- **工具**包括函数、MCP 服务器、`tool_search`（Claude）以及设置了 `mode: "disabled"` 的 `web_search`。支持情况取决于 harness 和 Environment；请参阅 [execution tools](../../../contracts/agents-api/zh/execution-tools.md)。

## Sessions（会话） {#sessions}

Session 是一次使用固定配置并拥有独立 Environment 的对话。

### 创建 Session {#create-a-session}

```python
session = client.beta.agents.sessions.create(
    environment={"type": "openai_hosted"},
    agent_id=agent.id,
    input="Review the files in /workspace and summarize the risks.",
    metadata={"ticket": "T-123"},
)
```

```sh
oac "/agents/sessions" -H "Idempotency-Key: $(uuidgen)" -d '{
  "environment": {"type": "openai_hosted"},
  "agent_id": "00000000-0000-4000-8000-000000000001",
  "input": "Review the files in /workspace and summarize the risks.",
  "metadata": {"ticket": "T-123"}
}'
```

返回 201 和 Session：

```json
{"id": "00000000-0000-4000-8000-000000000002", "object": "agent.session", "status": "idle",
 "agent": {"model": "...", ...}, "environment": {"id": "env_...", "type": "openai_hosted", ...},
 "metadata": {"ticket": "T-123"}, "required_actions": [], "vault_ids": [],
 "created_at": 1790000000, "last_active_at": 1790000000}
```

| 字段 | 含义 |
| --- | --- |
| `environment` | 必填。Agent 的工作位置；请参阅下表 |
| `agent_id` 或 `agent` | 已保存的 Agent，或内联 Agent 对象（字段与 Agent 创建操作相同）。`openai_hosted` 或 `none` 上的内联 Agent 可以省略 `model`，以使用安装的默认模型 |
| `input` | 第一条消息：字符串或消息数组。在 `none` 上为必填；在 `self_hosted` 之外使用 `stream: true` 时也为必填（[初始输入](../../../contracts/agents-api/zh/sessions-events.md#initial-input-at-session-creation)） |
| `metadata` | 你自己的字符串键值对 |
| `vault_ids` | MCP 服务器可使用其凭据的 [Vaults](#vaults) |
| `stream` | `true` 时返回[服务器发送事件](#stream-events)，而不是 JSON |
| `x_agents_core.model_provider` | 如果未从 Agent 或默认值继承，则指定此 Session 的模型访问方式。在 `none` 上会被拒绝 |

| `environment.type` | 运行位置 | 备注 |
| --- | --- | --- |
| `openai_hosted` | Core 在某个节点或 E2B 上创建的沙箱；容量由管理员提供 | 可选 `network`、`packages`、`files`、`skills`、`plugins`、`env`、`capability_directories`、`setup_commands`，也可指定模板 |
| `self_hosted` | 你自己的 Linux、macOS 或 Windows 计算机 | 要求提供绝对路径 `workspace_directory`。Skills、软件包、文件或模板应放在 `x_agents_core.environment` 中。响应会在 `x_agents_core.installation` 中携带安装命令；请参阅 [self-hosted execution](../getting-started/self-hosted.md)。Session 会自带自己的 `model_provider` |
| `none` | 由操作员注册的设备连接，无工作区 | `input` 为必填。模型来自安装的默认配置；如果未配置默认模型，则来自设备 |

新建的 `openai_hosted` Session 在 Core 准备沙箱期间会读取到 `idle`；Environment 准备就绪后，其首个 Turn 才会启动。[Environment contract](../../../contracts/agents-api/zh/environments.md) 负责部署位置、过期和准备过程。

### Session 状态 {#session-status}

读取 `status`、`error` 和 `required_actions`，以决定是发送输入、返回[函数结果](#function-tools)、连接计算机还是诊断故障。[Session status](../../../contracts/agents-api/zh/sessions-events.md#session-status) 定义了所有状态，以及哪些故障允许发送新输入。

### 更新、列出和删除 {#update-list-and-delete}

```python
client.beta.agents.sessions.update(session.id, metadata={"ticket": "T-124"})
for s in client.beta.agents.sessions.list(agent_id=agent.id):
    print(s.id)
client.beta.agents.sessions.delete(session.id)
```

| 操作 | HTTP | 备注 |
| --- | --- | --- |
| 读取 | `GET /agents/sessions/{id}` | |
| 更新 | `POST /agents/sessions/{id}` | 仅支持 `metadata` |
| 列出 | `GET /agents/sessions?agent_id=...` | 可选按 Agent 筛选 |
| 删除 | `DELETE /agents/sessions/{id}` | 仅当状态为 `idle` 或 `failed` 且没有待处理项时可用；否则返回 409。请先取消 |

## 发送输入 {#send-input}

所有输入都通过同一个端点以事件列表形式发送。输入持久化受理后、交给原生逻辑处理前，接口会返回 202。在空闲的 `openai_hosted` 或 `self_hosted` Session 上，消息请求最多可以等待五分钟，等待其 Turn 启动；请求可能因超时、取消或 Environment 错误而以 409 结束。客户端超时设置应涵盖这段等待时间（[Environment 输入](../../../contracts/agents-api/zh/sessions-events.md#sessions-with-an-environment)）。

### 发送消息 {#send-a-message}

```python
client.beta.agents.sessions.events.create(
    session.id,
    events=[{
        "type": "agent.session.input.message",
        "input": [{"role": "user", "content": [{"type": "input_text", "text": "Now fix the first risk."}]}],
    }],
    idempotency_key=key,
)
```

```sh
oac "/agents/sessions/$SESSION_ID/events" -H "Idempotency-Key: $KEY" -d '{
  "events": [{"type": "agent.session.input.message",
              "input": [{"role": "user", "content": [{"type": "input_text", "text": "Now fix the first risk."}]}]}]
}'
```

- **处于空闲状态时，**消息会启动一个新 Turn。**Turn 运行时，**消息会加入该 Turn（进行引导），而不会启动并行任务。
- **内容**使用 `input_text`，也可以使用 `input_image` 并以内联 PNG 或 JPEG 数据 URI 提供。Codex 和 Claude Code 接受图像；MiniMax Code 会拒绝图像。整个请求限制为 1 MiB。
- 完整规则请参阅[消息内容](../../../contracts/agents-api/zh/message-content.md)。

### 取消 {#cancel}

```python
client.beta.agents.sessions.events.create(session.id, events=[{"type": "agent.session.input.cancel"}])
```

```sh
oac "/agents/sessions/$SESSION_ID/events" -d '{"events": [{"type": "agent.session.input.cancel"}]}'
```

Turn 到达 `cancelled` 状态时才会被取消，而不是请求返回时立即取消。在空闲状态下发送取消时，如果没有待处理输入，则不会执行任何操作；如果 Environment 输入预留仍处于待处理状态，则返回 409。重启同一 installation 时，自托管计算机上的工作区和历史记录会保留；请参阅[操作 installation](../getting-started/self-hosted.md#operate-the-installation)。

## 流式传输事件 {#stream-events}

`GET /agents/sessions/{id}/events` 是服务器发送事件流。它**仅提供实时数据**：断连期间发送的事件不会重放。请在发送输入前打开该流，并通过 [Turns 和 Items](#turns-and-items) 补齐缺失内容。

```python
with client.beta.agents.sessions.events.stream(session.id) as stream:
    for event in stream:
        if event.type == "agent.session.turn.output_text.delta":
            print(event.delta, end="", flush=True)
        elif event.type in {"agent.session.turn.completed", "agent.session.turn.failed", "agent.session.turn.cancelled"}:
            break
```

```sh
oac "/agents/sessions/$SESSION_ID/events" -N
```

```text
event: agent.session.turn.output_text.delta
data: {"type": "agent.session.turn.output_text.delta", "item_id": "item_...", "delta": "Hello", ...}
```

| 事件 | 发生时机 |
| --- | --- |
| `agent.session.turn.created`、`.in_progress` | Turn 启动 |
| `agent.session.turn.item.added`、`.item.done` | Item（消息、工具调用等）开始或完成 |
| `agent.session.turn.output_text.delta`、`.done` | 逐段输出助手文本 |
| `agent.session.turn.completed`、`.failed`、`.cancelled` | Turn 结束。携带 `usage` |
| `agent.session.in_progress`、`.idle`、`.requires_action`、`.failed` | Session 状态发生变化 |
| `agent.session.subagent.*` | 子任务开始或结束 |
| `error` | 流级错误 |

该流会在多个 Turn 之间保持打开。要流式传输单个 Turn 并自动处理[函数调用](#function-tools)，SDK 的 `sessions.stream` 辅助函数可以同时完成这两项操作。

**流式传输创建过程本身：**在 `sessions.create` 中传入 `stream=True`。你会先收到 `agent.session.created`，流会在首次出现 `idle` 或 `failed` 时结束。

**重新连接：**重新订阅，然后读取 Items，并按 ID 丢弃已经拥有的 Item。当流所属的 Project 密钥被吊销或 Project 被归档时，打开的流会关闭。详细信息请参阅[恢复模型](../../../contracts/agents-api/zh/sessions-events.md#recovery-model)。

## Turns 和 Items（轮次和条目） {#turns-and-items}

Turn 是由输入启动的一项工作。Items 是其中记录的内容：消息、推理、工具调用及其结果。两者都是持久化的；你可以读取它们来检查结果或在断连后恢复。

```python
turns = client.beta.agents.sessions.turns.list(session.id, order="desc")
latest = turns.data[0]
print(latest.status, latest.usage)
for item in client.beta.agents.sessions.items.list(session.id, order="asc"):
    print(item.type)
```

```sh
oac "/agents/sessions/$SESSION_ID/turns?order=desc&limit=1"
oac "/agents/sessions/$SESSION_ID/items?order=asc"
```

| Turn `status` | 含义 |
| --- | --- |
| `queued`、`in_progress` | 尚未完成 |
| `waiting` | 正在等待函数结果 |
| `completed`、`failed`、`cancelled` | 已完成 |

- **用量**（`input_tokens`、`output_tokens`、`total_tokens`、……）未知时为 null，绝不会为零。Turn 运行期间，Session 的用量会保持为 null；Claude Code 和 MiniMax Code 不报告用量（[用量规则](../../../contracts/agents-api/zh/sessions-events.md#usage)）。
- Turn 列表只包含顶层 Turn。要读取 `/subagents` 下的子任务，请使用该路径。

## 函数工具 {#function-tools}

在 Agent 上声明函数。当模型调用该函数时，Session 会进入 `requires_action` 状态，Turn 会进入 `waiting` 状态，直到你返回结果。

```python
agent = client.beta.agents.create(
    model="your-model-id",
    tools=[{
        "type": "function",
        "name": "get_weather",
        "description": "Current weather for a city",
        "parameters": {"type": "object", "properties": {"city": {"type": "string"}}, "required": ["city"]},
    }],
)

def get_weather(args):
    return f"Sunny in {args['city']}"

session = client.beta.agents.sessions.create(
    environment={"type": "openai_hosted"}, agent_id=agent.id,
)

with client.beta.agents.sessions.stream(
    session.id, input="What's the weather in Paris?", tool_handlers={"get_weather": get_weather}
) as stream:
    for event in stream:
        pass  # the helper submits each result and stops when the Turn ends
```

如果不使用该辅助函数，请从 `session.required_actions` 读取调用，然后发送：

```sh
oac "/agents/sessions/$SESSION_ID/events" -d '{
  "events": [{"type": "agent.session.input.tool_result",
              "turn_id": "turn_...", "call_id": "call_...", "success": true, "output": "Sunny in Paris"}]
}'
```

重新发送相同结果是安全的。为同一次调用发送不同结果，或在取消后发送结果，都会失败并返回 409。MiniMax Code 不支持公共函数。

## Files（文件） {#files}

三种文件具有不同用途：

| 类型 | 用途 | 路径 |
| --- | --- | --- |
| [源文件](#source-files) | 一次性上传字节，并通过 ID 引用 | `/files` |
| [工作区文件](#workspace-files) | 将文件放入正在运行的 Session 工作区，或列出其中的文件 | `/agents/environments/{id}/files` |
| [Artifacts（产物）](#artifacts) | 下载 Agent 生成的内容 | `/agents/sessions/{id}/artifacts` |

### 源文件 {#source-files}

```python
f = client.files.create(file=open("data.csv", "rb"), purpose="user_data")
```

```sh
curl "$OPENAI_BASE_URL/files" -H "Authorization: Bearer $OPENAI_API_KEY" \
  -F purpose=user_data -F file=@data.csv
```

仅接受 `purpose=user_data`，最大为 512 MiB。不需要 Beta 请求头。其内容无法再次下载；请在工作区文件或模板中使用其 ID。

### 工作区文件 {#workspace-files}

将文件复制到 Session 的 Environment 工作区（`session.environment.id`）：

```python
env_id = session.environment.id
client.beta.agents.environments.files.create(env_id, type="file_id", file_id=f.id, path="/workspace/data.csv")
client.beta.agents.environments.files.create(env_id, type="inline", data="aGVsbG8K", path="/workspace/hello.txt")
page = client.beta.agents.environments.files.list(env_id, path="/workspace")
```

```sh
oac "/agents/environments/$ENV_ID/files" -d '{"type": "file_id", "file_id": "file-...", "path": "/workspace/data.csv"}'
```

- `inline` 数据使用 base64，解码后最大为 5 MiB；`file_id` 最大为 50 MiB。
- 系统会创建父目录。已有文件绝不会被覆盖（400）。
- 列表显示单个目录中的普通文件，不会递归列出内容。列表使用 `page` 令牌分页，并返回 `next`。

### Artifacts（产物） {#artifacts}

Turn 完成时，Core 会捕获工作区 `outputs/` 目录下的普通文件。即使 Environment 消失，Artifacts 仍可读取。

```python
for a in client.beta.agents.sessions.artifacts.list(session.id):
    data = client.beta.agents.sessions.artifacts.content(a.id, session_id=session.id)
    data.write_to_file(a.path.rsplit("/", 1)[-1])
```

```sh
oac "/agents/sessions/$SESSION_ID/artifacts"
oac "/agents/sessions/$SESSION_ID/artifacts/$ARTIFACT_ID/content" -o report.md
```

每个 Artifact 都有 `path`、`size_bytes`、`turn_id` 和 `environment_id`。删除 Artifact 不会删除工作区文件。

## Skills（技能） {#skills}

Skill 是 Agent 可使用的、包含指令和文件的带版本捆绑包。可以上传目录或 ZIP；每次上传都会创建一个版本。

```sh
curl "$OPENAI_BASE_URL/skills" -H "Authorization: Bearer $OPENAI_API_KEY" -F files=@my-skill.zip
```

```python
skill = client.skills.create(files=[("my-skill/SKILL.md", open("my-skill/SKILL.md", "rb"))])
client.skills.versions.create(skill.id, files=[...], default=True)
```

- 不需要 Beta 请求头。每个 Environment 最多可选择 50 个 Skills；每个归档压缩后最大为 5 MiB，展开后最大为 20 MiB。
- SDK 3.13.0 会上传内容中丢弃唯一的 ZIP 文件；上传 ZIP 时请使用 HTTP。
- 可通过 Session 的 `environment.skills` 或[模板](#environment-templates)为其附加 Skills。

详细信息请参阅 [Files and Skills](../../../contracts/agents-api/zh/source-files.md)。Session 会在准备期间一次性安装其 Skills、Plugins 和软件包；之后编辑源文件不会影响正在运行的 Session。准备错误会在任何工作开始前导致 Session 失败：请修复原因，而不是在新 Session 中重试。

## Environment Templates（环境模板） {#environment-templates}

模板用于保存工作区设置以供复用。`openai_hosted` Session 在 `environment` 中引用模板；`self_hosted` Session 则在 `x_agents_core.environment` 中引用：

```python
template = client.beta.agents.environments.templates.create(
    name="python-data",
    packages={"python": ["pandas"]},
    setup_commands=[{"command": "mkdir -p /workspace/outputs"}],
)
```

| 字段 | 含义 |
| --- | --- |
| `network` | `access` 可设为 `enabled`（默认值）、`disabled` 或 `restricted`；设为 `restricted` 时，可将网络限制为 `allowed_domains` 中 1–100 个精确主机。Session 只能进一步缩小范围。请参阅[受限网络策略](../../../contracts/agents-api/zh/environments.md#restricted-network)中的执行限制 |
| `packages` | 软件包设置；请参阅[软件包准入](../../../contracts/agents-api/zh/environments.md#preparation-order) |
| `setup_commands`、`env` | 在准备期间运行和设置。读取时从不返回 |
| `files`、`skills`、`plugins` | 初始内容。最多 50 个文件，内联数据总计最多 10 MiB |

Session 启动时会冻结模板。详细信息请参阅 [Environment Templates](../../../contracts/agents-api/zh/environments.md#templates)。

## Vaults（凭据库） {#vaults}

Vaults 保存 HTTP MCP 服务器的凭据：`static_bearer` 令牌，或支持可选刷新的 `mcp_oauth` 令牌。令牌为只写字段。

```python
vault = client.beta.agents.vaults.create(name="github")
client.beta.agents.vaults.credentials.create(
    vault.id, name="github-token",
    auth={"type": "static_bearer", "token": "ghp_...", "mcp_server_url": "https://api.githubcopilot.com/mcp/"},
)
session = client.beta.agents.sessions.create(environment={"type": "none"}, input="...", vault_ids=[vault.id], agent_id=agent.id)
```

HTTP 路径为 `/vaults`，需要 Beta 请求头。Session 从其 `vault_ids` 中选择凭据，也可以通过 `credential_id` 指定具体凭据；无论采用哪种方式，MCP 服务器的 URL 都必须与所选凭据的 `mcp_server_url` 完全匹配。[Vaults contract](../../../contracts/agents-api/zh/vaults.md) 负责选择、错误、OAuth 刷新和删除。MCP 工具的 `connection_origin` 决定由 Core 端还是工作区连接服务器，而且每个 harness 支持的取值集合不同；请参阅 [MCP connection origin](../../../contracts/agents-api/zh/environments.md#public-mcp-connection-origin)。

## 完整示例 {#worked-examples}

以下示例将前文介绍的各种资源组合成完整流程。它们使用固定版本的 SDK 和 Core `/v1` 端点；请将 `your-model-id` 替换为你的安装所提供的模型 ID。可运行版本位于 [`example/hosted-agents-python`](https://github.com/MiniMax-AI/OpenAgentCore/blob/main/example/hosted-agents-python/README.md)。

### 异步客户端与异步工具处理器 {#async-client-and-async-tool-handlers}

```python
import asyncio

from openai import AsyncOpenAI

client = AsyncOpenAI()  # reads OPENAI_API_KEY and OPENAI_BASE_URL

async def get_weather(args: dict) -> str:
    return f"Sunny in {args['city']}"

async def main() -> None:
    agent = await client.beta.agents.create(
        model="your-model-id",
        name="Weather assistant",
        tools=[{
            "type": "function",
            "name": "get_weather",
            "description": "Current weather for a city",
            "parameters": {"type": "object", "properties": {"city": {"type": "string"}}, "required": ["city"]},
        }],
    )
    session = await client.beta.agents.sessions.create(
        environment={"type": "openai_hosted"},
        agent_id=agent.id,
    )
    async with client.beta.agents.sessions.stream(
        session.id,
        input="What's the weather in Paris?",
        tool_handlers={"get_weather": get_weather},
    ) as stream:
        async for event in stream:
            if event.type == "agent.session.turn.output_text.delta":
                print(event.delta, end="", flush=True)

asyncio.run(main())
```

每个资源都有参数相同的 `Async*` 对应实现，异步 `sessions.stream` helper 同时接受普通 handler 结果和可等待的 handler 结果。

### 流式创建 Session {#stream-the-session-creation}

```python
with client.beta.agents.sessions.create(
    environment={"type": "openai_hosted"},
    agent_id=agent.id,
    input="Review the repository and summarize the risks.",
    stream=True,
) as stream:
    for event in stream:
        if event.type == "agent.session.created":
            print("session", event.session.id)
        elif event.type == "agent.session.environment.ready":
            print("environment ready")
        elif event.type == "agent.session.idle":
            break
```

`stream=True` 只改变响应类型：同一个 POST 返回 `Stream[AgentSessionEvent]` 而不是 `AgentSession`。该流以 `agent.session.created` 开始，并在第一个 `idle` 或 `failed` 处结束；之后读取 Session 以了解结果（[流式事件](#stream-events)）。

取消仍在运行的 Turn：

```python
client.beta.agents.sessions.events.create(
    session.id,
    events=[{"type": "agent.session.input.cancel"}],
)
```

Turn 稍后才会到达 `cancelled` 状态；此请求返回并不意味着它已经停止（[取消](#cancel)）。

### 文件、工作区文件和 Artifacts {#files-workspace-files-and-artifacts}

```python
import base64
import time

with open("data.csv", "rb") as data_file:
    source = client.files.create(file=data_file, purpose="user_data")

env_id = session.environment.id

# A hosted Environment rejects workspace file operations while it is `pending`.
while True:
    status = client.beta.agents.environments.retrieve(env_id).status
    if status == "connected":
        break
    if status in {"failed", "expired", "disconnected"}:
        raise SystemExit(f"environment is {status}")
    time.sleep(3)

client.beta.agents.environments.files.create(
    env_id, type="file_id", file_id=source.id, path="/workspace/data.csv",
)
client.beta.agents.environments.files.create(
    env_id,
    type="inline",
    data=base64.b64encode(b"hello\n").decode(),
    path="/workspace/hello.txt",
)

page = client.beta.agents.environments.files.list(env_id, path="/workspace", limit=100)
while True:
    for workspace_file in page.data:
        print(workspace_file.path, workspace_file.size_bytes)
    if page.next is None:
        break
    page = client.beta.agents.environments.files.list(
        env_id, path="/workspace", limit=100, page=page.next,
    )

# After a Turn completes, download what it wrote under outputs/.
for artifact in client.beta.agents.sessions.artifacts.list(session.id):
    content = client.beta.agents.sessions.artifacts.content(artifact.id, session_id=session.id)
    content.write_to_file(artifact.path.rsplit("/", 1)[-1])
```

Source File 只保存一次字节；工作区文件可以按 ID（`file_id`）复制 Source File，也可以携带 base64 `inline` 数据。工作区列表使用不透明的 `page` token 分页，而不是其他列表使用的 `after` 游标。Turn 完成时，Core 会从工作区的 `outputs/` 目录捕获 Artifacts（[Environment files and Artifacts](../../../contracts/agents-api/zh/environment-files.md)）。

### Skills 与环境模板 {#skills-and-an-environment-template}

```python
with open("my-skill/SKILL.md", "rb") as skill_file:
    skill = client.skills.create(files=[("my-skill/SKILL.md", skill_file)])

with open("my-skill/SKILL.md", "rb") as skill_file:
    client.skills.versions.create(
        skill.id, files=[("my-skill/SKILL.md", skill_file)], default=True,
    )

template = client.beta.agents.environments.templates.create(
    name="python-data",
    packages={"python": ["pandas"]},
    setup_commands=[{"command": "mkdir -p /workspace/outputs"}],
    skills=[{"type": "skill_reference", "skill_id": skill.id}],
)

session = client.beta.agents.sessions.create(
    environment={"type": "openai_hosted", "environment_template_id": template.id},
    agent_id=agent.id,
)
```

Skills 通过顶层 `client.skills` 资源上传（不需要 Beta 请求头），每次上传都是一个版本；请以 `(name, file)` 元组传入文件。SDK 3.13.0 不会上传单个 ZIP 文件，因此 ZIP 归档请使用 HTTP（[Skills](#skills)）。Session 通过 `environment_template_id` 引用模板，并在启动时将其冻结（[环境模板](#environment-templates)）。

### Vaults、MCP 服务器与凭据 {#vaults-mcp-servers-and-credentials}

```python
vault = client.beta.agents.vaults.create(name="internal")
client.beta.agents.vaults.credentials.create(
    vault.id,
    name="Internal MCP",
    auth={
        "type": "static_bearer",
        "mcp_server_url": "https://mcp.example.com/endpoint",
        "token": token_from_private_configuration,
    },
)

session = client.beta.agents.sessions.create(
    environment={"type": "openai_hosted"},
    vault_ids=[vault.id],
    input="List the open incidents.",
    agent={
        "model": "your-model-id",
        "tools": [{
            "type": "mcp",
            "server_label": "internal",
            "transport": {"type": "http", "server_url": "https://mcp.example.com/endpoint"},
            "connection_origin": "environment",
        }],
    },
)
```

MCP 工具可以用 `credential_id` 指定 Credential；未指定时，会选择 `mcp_server_url` 与工具的 `server_url` 相等的已附加 Credential（[Vaults 与凭据](../../../contracts/agents-api/zh/vaults.md#credential-selection-in-a-session)）。`environment` origin 运行在 `openai_hosted` 上；`service` 需要 `none` 部署位置，而且每个 harness 支持的取值集合不同（[MCP connection origin](../../../contracts/agents-api/zh/environments.md#public-mcp-connection-origin)）。

### 自托管 Session {#self-hosted-sessions}

`self_hosted` Session 及其安装命令见[自托管执行](../getting-started/self-hosted.md#connect-a-machine)。Session 通过 `extra_body` 中的 `x_agents_core.model_provider` 自带模型提供商；协议必须与 harness 匹配（[Model execution](../../../contracts/agents-api/zh/model-execution.md)）。

### 分页、幂等性与类型化错误 {#pagination-idempotency-and-typed-errors}

```python
import uuid

from openai import ConflictError, NotFoundError

for session in client.beta.agents.sessions.list(limit=100):
    print(session.id, session.status)

creation_key = str(uuid.uuid4())  # store it before the first attempt
session = client.beta.agents.sessions.create(
    environment={"type": "openai_hosted"},
    agent_id=agent.id,
    extra_headers={"Idempotency-Key": creation_key},
)

client.beta.agents.sessions.events.create(
    session.id,
    events=[{
        "type": "agent.session.input.message",
        "input": [{"role": "user", "content": [{"type": "input_text", "text": "Continue."}]}],
    }],
    idempotency_key=str(uuid.uuid4()),
)

try:
    client.beta.agents.sessions.delete(session.id)
except ConflictError as error:
    print(error.code, error.param, error.request_id)
except NotFoundError:
    print("already gone")
```

列表方法在迭代时会自动分页。`sessions.create` 没有名为 `idempotency_key` 的参数；请通过 `extra_headers={"Idempotency-Key": ...}` 传入，并在重试时复用同一个键（[幂等性](#idempotency)）。API 失败会以类型化异常的形式返回，携带 `.code`、`.param`、`.type` 和 `.request_id`（[错误](#errors)）。

## 诊断故障 {#diagnose-a-failure}

1. 读取 Session 的 `status` 和 `error`，以及最新 Turn 的 `error`。失败的 Turn 只会报告通用的 `internal_error`。
2. 检查 Environment 是否已连接，以及其 harness 是否可用。
3. 在 [Harness capabilities](../../../contracts/agents-api/zh/harness-capabilities.md) 中检查 harness、模型和工具的组合。
4. 向管理员索取 Session 的[诊断信息](../../../contracts/agents-api/zh/session-diagnostics.md)，其中会指出故障类别；同时请查看[故障排除](../getting-started/operations.md#troubleshooting)，了解服务日志、凭据和节点就绪状态。

401 通常表示使用了其他命名空间中的密钥；请参阅 [API 命名空间与凭据](index.md)。

## 与 OpenAI 的差异 {#differences-from-openai}

Core 在某些行为上与 OpenAI 服务不同，例如 Session 创建的幂等性和特定 harness 的工具支持。[覆盖情况清单](../../../contracts/agents-api/zh/index.md#differences-from-openai) 列出了所有差异及各资源的状态；[公共 OpenAPI](../../../contracts/agents-api/openapi.yaml) 包含准确的模式定义。
