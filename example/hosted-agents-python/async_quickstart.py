#!/usr/bin/env python3
"""The quickstart with the async client and an async tool handler.

Every resource has an ``Async*`` counterpart with the same parameters, and the
async ``sessions.stream`` helper accepts awaitable handler results.
"""

from __future__ import annotations

import asyncio
import os
import sys

from openai import AsyncOpenAI

REQUIRED_ENV = ("OPENAI_BASE_URL", "OPENAI_API_KEY", "OAC_MODEL")


async def get_weather(args: dict) -> str:
    return f"Sunny in {args['city']}"


async def main() -> None:
    for name in REQUIRED_ENV:
        if not os.environ.get(name):
            sys.exit(f"Set {name}; see example/hosted-agents-python/README.md.")

    client = AsyncOpenAI()  # reads OPENAI_BASE_URL and OPENAI_API_KEY

    agent = await client.beta.agents.create(
        model=os.environ["OAC_MODEL"],
        name="Async weather assistant",
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

    session = await client.beta.agents.sessions.create(
        environment={"type": "openai_hosted"},
        agent_id=agent.id,
    )
    print(f"session: {session.id}")

    print("assistant: ", end="", flush=True)
    async with client.beta.agents.sessions.stream(
        session.id,
        input="What's the weather in Paris?",
        tool_handlers={"get_weather": get_weather},
    ) as stream:
        async for event in stream:
            if event.type == "agent.session.turn.output_text.delta":
                print(event.delta, end="", flush=True)
    print()


if __name__ == "__main__":
    asyncio.run(main())
