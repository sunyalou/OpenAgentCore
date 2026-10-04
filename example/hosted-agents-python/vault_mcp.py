#!/usr/bin/env python3
"""Store an MCP credential in a Vault and attach the server to a Session.

The Vault is attached with ``vault_ids``; the MCP tool names the server by URL,
and Core selects the attached Credential whose ``mcp_server_url`` matches it.
The default ``environment`` origin connects from the Session's workspace on an
``openai_hosted`` Environment. Set ``OAC_MCP_PROMPT`` to also run a Turn
against the server.
"""

from __future__ import annotations

import os
import sys

from openai import OpenAI

REQUIRED_ENV = (
    "OPENAI_BASE_URL",
    "OPENAI_API_KEY",
    "OAC_MODEL",
    "OAC_MCP_URL",
    "OAC_MCP_TOKEN",
)


def main() -> None:
    for name in REQUIRED_ENV:
        if not os.environ.get(name):
            sys.exit(f"Set {name}; see example/hosted-agents-python/README.md.")

    client = OpenAI()
    mcp_url = os.environ["OAC_MCP_URL"]

    vault = client.beta.agents.vaults.create(name="example-mcp")
    credential = client.beta.agents.vaults.credentials.create(
        vault.id,
        name="Example MCP",
        auth={
            "type": "static_bearer",
            "mcp_server_url": mcp_url,
            "token": os.environ["OAC_MCP_TOKEN"],
        },
    )
    print(f"vault: {vault.id}")
    print(f"credential: {credential.id}")

    session = client.beta.agents.sessions.create(
        environment={"type": "openai_hosted"},
        vault_ids=[vault.id],
        agent={
            "model": os.environ["OAC_MODEL"],
            "instructions": "Use the attached MCP server when it helps.",
            "tools": [
                {
                    "type": "mcp",
                    "server_label": "example",
                    "transport": {"type": "http", "server_url": mcp_url},
                    "connection_origin": "environment",
                }
            ],
        },
    )
    print(f"session: {session.id}")

    prompt = os.environ.get("OAC_MCP_PROMPT")
    if not prompt:
        print("set OAC_MCP_PROMPT to also send a Turn against the MCP server")
        return

    print("assistant: ", end="", flush=True)
    with client.beta.agents.sessions.stream(session.id, input=prompt) as stream:
        for event in stream:
            if event.type == "agent.session.turn.output_text.delta":
                print(event.delta, end="", flush=True)
    print()


if __name__ == "__main__":
    main()
