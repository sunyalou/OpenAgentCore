#!/usr/bin/env python3
"""Override a saved Agent with a Session-level inline agent.

Session creation accepts ``agent_id`` and an inline ``agent``: inline fields
replace the saved field whole, omitted fields inherit, and the Session
snapshots the result. ``x_agents_core`` is a Core extension, so the inline
agent goes through ``extra_body``.
"""

from __future__ import annotations

import os
import sys

from openai import OpenAI

REQUIRED_ENV = ("OPENAI_BASE_URL", "OPENAI_API_KEY", "OAC_MODEL")

SAVED_INSTRUCTIONS = (
    "When asked which configuration you belong to, answer only: saved-agent."
)
INLINE_INSTRUCTIONS = (
    "When asked which configuration you belong to, answer only: inline-override."
)


def main() -> None:
    for name in REQUIRED_ENV:
        if not os.environ.get(name):
            sys.exit(f"Set {name}; see example/hosted-agents-python/README.md.")

    model = os.environ["OAC_MODEL"]
    harness = os.environ.get("OAC_HARNESS", "codex")

    client = OpenAI()  # reads OPENAI_BASE_URL and OPENAI_API_KEY

    saved = client.beta.agents.create(
        model=model,
        name="Inline override example (base)",
        instructions=SAVED_INSTRUCTIONS,
        extra_body={"x_agents_core": {"harness": harness}},
    )
    print(f"saved agent: {saved.id}")

    session = client.beta.agents.sessions.create(
        environment={"type": "openai_hosted"},
        agent_id=saved.id,
        extra_body={
            "agent": {
                "model": model,
                "instructions": INLINE_INSTRUCTIONS,
                "x_agents_core": {"harness": harness},
            }
        },
    )
    print(f"session: {session.id} ({session.status})")
    print(f"effective instructions: {session.agent.instructions!r}")

    print("assistant: ", end="", flush=True)
    with client.beta.agents.sessions.stream(
        session.id,
        input="Which configuration do you belong to?",
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

    unchanged = client.beta.agents.retrieve(saved.id)
    print(f"saved agent after the session: {unchanged.instructions!r}")


if __name__ == "__main__":
    main()
