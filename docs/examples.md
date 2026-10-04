---
title: "Examples"
---

Applications and scripts built on the [Agents API](./api/public-agent-api.md). Each one runs against a real Core installation with a Project API key.

| Example | What it shows |
| --- | --- |
| [Parsar Agent workbench](#parsar-agent-workbench) | A product UI: models, Skills, MCP, reusable Agents, and Sessions on any runtime |
| [Hosted Agents Python scripts](#hosted-agents-python-scripts) | Eight small scripts for the hosted Agents API, from quickstart to Vaults and MCP |

## Parsar Agent workbench

A small single-user product built on Core. You save model providers, Skills and MCP servers, combine them into Agents, then start Sessions in a managed sandbox, with no workspace, or on your own machine.

Source: [`example/parsar`](https://github.com/MiniMax-AI/OpenAgentCore/blob/main/example/parsar/README.md).

Run and configure it with the [example README](https://github.com/MiniMax-AI/OpenAgentCore/blob/main/example/parsar/README.md#run), which also owns its supported features, storage and validation.

### What to look at

Each feature maps to one part of the API. Read the code next to the guide section:

| Feature | API used | Guide | Code |
| --- | --- | --- | --- |
| Server-side key and headers | Bearer key, `OpenAI-Beta`, route allowlist | [Before you start](./api/public-agent-api.md#before-you-start) | [`server.mjs`](https://github.com/MiniMax-AI/OpenAgentCore/blob/main/example/parsar/server.mjs) |
| Agents with a harness and model | `x_agents_core.harness`, `model_provider` | [Core extensions](./api/public-agent-api.md#core-extensions-x_agents_core) | [`server/sessions.mjs`](https://github.com/MiniMax-AI/OpenAgentCore/blob/main/example/parsar/server/sessions.mjs) |
| Sessions on three runtimes | `openai_hosted`, `none`, `self_hosted` with `workspace_directory` | [Create a Session](./api/public-agent-api.md#create-a-session) | [`server/sessions.mjs`](https://github.com/MiniMax-AI/OpenAgentCore/blob/main/example/parsar/server/sessions.mjs) |
| Crash-safe Session creation | `Idempotency-Key`, recovery from a lost response | [Idempotency](./api/public-agent-api.md#idempotency) | [`server/sessions.mjs`](https://github.com/MiniMax-AI/OpenAgentCore/blob/main/example/parsar/server/sessions.mjs) |
| Live replies | Event stream, text deltas, reconnect and reconcile | [Stream events](./api/public-agent-api.md#stream-events) | [`src/lib/live-session.ts`](https://github.com/MiniMax-AI/OpenAgentCore/blob/main/example/parsar/src/lib/live-session.ts) |
| Follow-ups and cancel | Input events with idempotency keys | [Send input](./api/public-agent-api.md#send-input) | [`src/Composer.tsx`](https://github.com/MiniMax-AI/OpenAgentCore/blob/main/example/parsar/src/Composer.tsx) |
| History | Paging Turns and Items | [Pagination](./api/public-agent-api.md#pagination) | [`src/lib/api.ts`](https://github.com/MiniMax-AI/OpenAgentCore/blob/main/example/parsar/src/lib/api.ts) |
| Skills and versions | `/skills`, default version | [Skills](./api/public-agent-api.md#skills) | [`src/Skills.tsx`](https://github.com/MiniMax-AI/OpenAgentCore/blob/main/example/parsar/src/Skills.tsx) |
| Connect your own machine | `x_agents_core.installation`, Environment status | [Self-hosted execution](./getting-started/self-hosted.md) | [`src/ConnectMachine.tsx`](https://github.com/MiniMax-AI/OpenAgentCore/blob/main/example/parsar/src/ConnectMachine.tsx) |

## Hosted Agents Python scripts

Small standalone scripts for the hosted Agents API, one per flow: quickstart, async, streaming and cancellation, files and Artifacts, Skills and templates, Vaults and MCP, self-hosted Sessions, and pagination, idempotency and typed errors.

Source: [`example/hosted-agents-python`](https://github.com/MiniMax-AI/OpenAgentCore/blob/main/example/hosted-agents-python/README.md).

Run and configure them with the [example README](https://github.com/MiniMax-AI/OpenAgentCore/blob/main/example/hosted-agents-python/README.md#requirements), which owns its requirements and notes. Each script prints the resources it creates and demonstrates the calls in the [worked examples](./api/public-agent-api.md#worked-examples).

## Add an example

Create a directory under `example/` with a README explaining how to run it and which API features it demonstrates, then add a row to this index. Follow the [application example boundary](https://github.com/MiniMax-AI/OpenAgentCore/blob/main/CONTRIBUTING.md#optional-application-example).
