#!/usr/bin/env python3
"""Stream a Session creation, then stream and cancel a running Turn.

Part one shows the ``stream=True`` creation stream: ``agent.session.created``
first, then Environment and Turn events, ending at the first ``idle`` or
``failed``. Part two subscribes to the live event stream, sends input and
cancels the Turn after its first text delta.
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

    client = OpenAI()

    agent = client.beta.agents.create(
        model=os.environ["OAC_MODEL"],
        name="Streaming demo",
        instructions="Follow instructions exactly.",
    )
    print(f"agent: {agent.id}")

    print("creating a Session with stream=True")
    session_id = None
    with client.beta.agents.sessions.create(
        environment={"type": "openai_hosted"},
        agent_id=agent.id,
        input="Reply with exactly: ready",
        stream=True,
    ) as stream:
        for event in stream:
            print(f"  create: {event.type}")
            if event.type == "agent.session.created":
                session_id = event.session.id
            elif event.type == "agent.session.idle":
                break
    print(f"session: {session_id}")

    print("streaming a Turn and cancelling it after the first delta")
    with client.beta.agents.sessions.events.stream(session_id) as stream:
        client.beta.agents.sessions.events.create(
            session_id,
            events=[
                {
                    "type": "agent.session.input.message",
                    "input": [
                        {
                            "role": "user",
                            "content": [
                                {
                                    "type": "input_text",
                                    "text": "Count from 1 to 200, one number per line.",
                                }
                            ],
                        }
                    ],
                }
            ],
        )
        cancel_sent = False
        for event in stream:
            if event.type == "agent.session.turn.output_text.delta":
                print(event.delta, end="", flush=True)
                if not cancel_sent:
                    client.beta.agents.sessions.events.create(
                        session_id,
                        events=[{"type": "agent.session.input.cancel"}],
                    )
                    cancel_sent = True
            elif event.type == "agent.session.turn.cancelled":
                print("\nturn cancelled")
                break
            elif event.type == "agent.session.turn.completed":
                print("\nturn completed before the cancel arrived")
                break
            elif event.type == "agent.session.failed":
                print("\nsession failed")
                break


if __name__ == "__main__":
    main()
