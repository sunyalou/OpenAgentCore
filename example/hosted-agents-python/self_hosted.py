#!/usr/bin/env python3
"""Create a self-hosted Session and print the command that connects the machine.

The Session brings its own model provider through ``x_agents_core`` because a
self-hosted machine never uses the deployment model settings. The protocol must
match the harness: Codex uses ``responses``, Claude SDK uses ``anthropic``, and
MiniMax Code accepts ``anthropic``, ``responses`` or ``chat_completions``.
"""

from __future__ import annotations

import os
import sys

from openai import OpenAI

REQUIRED_ENV = (
    "OPENAI_BASE_URL",
    "OPENAI_API_KEY",
    "OAC_MODEL",
    "OAC_WORKSPACE",
    "OAC_MODEL_BASE_URL",
    "OAC_MODEL_API_KEY",
)


def main() -> None:
    for name in REQUIRED_ENV:
        if not os.environ.get(name):
            sys.exit(f"Set {name}; see example/hosted-agents-python/README.md.")

    harness = os.environ.get("OAC_HARNESS", "codex")
    protocol = os.environ.get("OAC_MODEL_PROTOCOL", "responses")

    client = OpenAI()
    session = client.beta.agents.sessions.create(
        environment={
            "type": "self_hosted",
            "workspace_directory": os.environ["OAC_WORKSPACE"],
        },
        extra_body={
            "agent": {
                "model": os.environ["OAC_MODEL"],
                "x_agents_core": {"harness": harness},
            },
            "x_agents_core": {
                "model_provider": {
                    "protocol": protocol,
                    "base_url": os.environ["OAC_MODEL_BASE_URL"],
                    "api_key": os.environ["OAC_MODEL_API_KEY"],
                },
            },
        },
    )
    print(f"session: {session.id}")

    installation = session.model_dump()["x_agents_core"]["installation"]
    print("run this on the target machine:")
    print(installation["commands"]["posix"])  # "powershell" on Windows

    environment = client.beta.agents.environments.retrieve(session.environment.id)
    print(f"environment: {environment.id} ({environment.status})")
    print("after the command completes, the environment status becomes connected.")


if __name__ == "__main__":
    main()
