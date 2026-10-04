#!/usr/bin/env python3
"""Pagination, idempotent creation and typed errors.

List methods auto-paginate when iterated. A repeated Session creation with the
same ``Idempotency-Key`` returns the original Session instead of creating a
second one, a Core difference from the official service. API failures arrive as
typed exceptions carrying ``type``, ``code``, ``param`` and ``request_id``.
"""

from __future__ import annotations

import os
import sys
import uuid

from openai import ConflictError, NotFoundError, OpenAI

REQUIRED_ENV = ("OPENAI_BASE_URL", "OPENAI_API_KEY", "OAC_MODEL")


def main() -> None:
    for name in REQUIRED_ENV:
        if not os.environ.get(name):
            sys.exit(f"Set {name}; see example/hosted-agents-python/README.md.")

    client = OpenAI()

    print("agents:")
    for agent in client.beta.agents.list(limit=100):
        print(f"  {agent.id} {agent.name}")
    print("sessions:")
    for session in client.beta.agents.sessions.list(limit=100):
        print(f"  {session.id} {session.status}")

    print("retrieving a missing session:")
    try:
        client.beta.agents.sessions.retrieve("00000000-0000-4000-8000-000000000000")
    except NotFoundError as error:
        print(f"  type={error.type} code={error.code} request_id={error.request_id}")

    print("creating a Session twice with the same Idempotency-Key:")
    key = str(uuid.uuid4())
    body = {
        "environment": {"type": "openai_hosted"},
        "agent": {"model": os.environ["OAC_MODEL"]},
    }
    first = client.beta.agents.sessions.create(**body, extra_headers={"Idempotency-Key": key})
    second = client.beta.agents.sessions.create(**body, extra_headers={"Idempotency-Key": key})
    print(f"  first:  {first.id}")
    print(f"  second: {second.id}")
    print(f"  same session: {first.id == second.id}")

    try:
        client.beta.agents.sessions.delete(first.id)
        print("  deleted the Session")
    except ConflictError as error:
        print(f"  delete conflicted: {error.code}")


if __name__ == "__main__":
    main()
