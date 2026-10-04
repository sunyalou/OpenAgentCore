#!/usr/bin/env python3
"""Create an Agent with a function tool and run one Turn.

The ``sessions.stream`` helper subscribes to the event stream, submits the
input, runs the registered tool handlers and returns when the Turn ends.
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

    client = OpenAI()  # reads OPENAI_BASE_URL and OPENAI_API_KEY

    agent = client.beta.agents.create(
        model=os.environ["OAC_MODEL"],
        name="Quickstart weather assistant",
        instructions="Answer weather questions with the get_weather tool.",
        tools=[
            {
                "type": "function",
                "name": "get_weather",
                "description": "Current weather for a city",
                "parameters": {
                    "type": "object",
                    "properties": {"city": {"type": "string"}},
                    "required": ["city"],
                },
            }
        ],
    )
    print(f"agent: {agent.id}")

    session = client.beta.agents.sessions.create(
        environment={"type": "openai_hosted"},
        agent_id=agent.id,
    )
    print(f"session: {session.id}")

    def get_weather(args: dict) -> str:
        return f"Sunny in {args['city']}"

    print("assistant: ", end="", flush=True)
    with client.beta.agents.sessions.stream(
        session.id,
        input="What's the weather in Paris?",
        tool_handlers={"get_weather": get_weather},
    ) as stream:
        for event in stream:
            if event.type == "agent.session.turn.output_text.delta":
                print(event.delta, end="", flush=True)
            elif event.type == "agent.session.turn.completed":
                print()
                if event.usage is not None:
                    print(f"usage: {event.usage.total_tokens} tokens")

    turn = client.beta.agents.sessions.turns.list(session.id, order="desc", limit=1).data[0]
    print(f"turn: {turn.id} {turn.status}")
    for item in client.beta.agents.sessions.items.list(session.id, order="asc"):
        print(f"item: {item.type}")


if __name__ == "__main__":
    main()
