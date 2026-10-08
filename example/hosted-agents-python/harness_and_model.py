#!/usr/bin/env python3
"""Create a saved Agent with an explicit harness and model, then run one Turn.

``x_agents_core`` is a Core extension, so it goes through ``extra_body``: the
harness is ``x_agents_core.harness`` and the model is the official ``model``
field. Without an explicit harness, the installation's default applies.
"""

from __future__ import annotations

import os
import sys

from openai import OpenAI

REQUIRED_ENV = ("OPENAI_BASE_URL", "OPENAI_API_KEY", "OAC_MODEL")


def main() -> None:
    for name in REQUIRED_ENV:
        if not os.environ.get(name):
            sys.exit(f"Set {name}; see example/hosted-agents-python/README.md.")

    model = os.environ["OAC_MODEL"]
    harness = os.environ.get("OAC_HARNESS", "codex")

    client = OpenAI()  # reads OPENAI_BASE_URL and OPENAI_API_KEY

    agent = client.beta.agents.create(
        model=model,
        name="Harness and model example",
        instructions="Answer in one short sentence.",
        extra_body={"x_agents_core": {"harness": harness}},
    )
    print(f"agent: {agent.id} (harness={harness}, model={model})")

    session = client.beta.agents.sessions.create(
        environment={"type": "openai_hosted"},
        agent_id=agent.id,
    )
    print(f"session: {session.id} ({session.status})")

    print("assistant: ", end="", flush=True)
    with client.beta.agents.sessions.stream(
        session.id,
        input="In one sentence, introduce yourself.",
    ) as stream:
        for event in stream:
            if event.type == "agent.session.turn.output_text.delta":
                print(event.delta, end="", flush=True)
            elif event.type == "agent.session.turn.completed":
                print()
                if event.usage is not None:
                    print(f"usage: {event.usage.total_tokens} tokens")

    turn = client.beta.agents.sessions.turns.list(
        session.id, order="desc", limit=1
    ).data[0]
    print(f"turn: {turn.id} ({turn.status})")


if __name__ == "__main__":
    main()
