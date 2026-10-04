#!/usr/bin/env python3
"""Upload a Source File, place workspace files, and download the Turn's Artifacts.

Workspace file operations return 400 while a hosted Environment is still
``pending``, so the script waits until ``environments.retrieve`` reports
``connected`` before writing files. When the Turn completes, Core captures the
regular files under ``/workspace/outputs`` as Artifacts.
"""

from __future__ import annotations

import base64
import os
import sys
import tempfile
import time

from openai import OpenAI

REQUIRED_ENV = ("OPENAI_BASE_URL", "OPENAI_API_KEY", "OAC_MODEL")


def wait_until_connected(client: OpenAI, environment_id: str, timeout_seconds: int = 900) -> None:
    deadline = time.monotonic() + timeout_seconds
    while True:
        status = client.beta.agents.environments.retrieve(environment_id).status
        if status == "connected":
            return
        if status in {"failed", "expired", "disconnected"}:
            sys.exit(f"Environment {environment_id} is {status}.")
        if time.monotonic() > deadline:
            sys.exit(f"Environment {environment_id} was not connected within {timeout_seconds}s.")
        time.sleep(3)


def main() -> None:
    for name in REQUIRED_ENV:
        if not os.environ.get(name):
            sys.exit(f"Set {name}; see example/hosted-agents-python/README.md.")

    client = OpenAI()

    agent = client.beta.agents.create(
        model=os.environ["OAC_MODEL"],
        name="Files demo",
        instructions="Read files from /workspace and write requested outputs under /workspace/outputs.",
    )
    session = client.beta.agents.sessions.create(
        environment={"type": "openai_hosted"},
        agent_id=agent.id,
    )
    env_id = session.environment.id
    print(f"session: {session.id}")
    print(f"environment: {env_id}")

    wait_until_connected(client, env_id)

    with tempfile.NamedTemporaryFile("w", suffix=".csv", delete=False) as data_file:
        data_file.write("city,visits\nParis,3\nBerlin,1\n")
        data_path = data_file.name
    try:
        with open(data_path, "rb") as upload:
            source = client.files.create(file=upload, purpose="user_data")
    finally:
        os.unlink(data_path)
    print(f"source file: {source.id}")

    client.beta.agents.environments.files.create(
        env_id,
        type="file_id",
        file_id=source.id,
        path="/workspace/inputs/visits.csv",
    )
    client.beta.agents.environments.files.create(
        env_id,
        type="inline",
        data=base64.b64encode(b"Write the summary as Markdown.\n").decode(),
        path="/workspace/inputs/notes.txt",
    )

    print("assistant: ", end="", flush=True)
    with client.beta.agents.sessions.stream(
        session.id,
        input=(
            "Read /workspace/inputs/visits.csv and /workspace/inputs/notes.txt, "
            "then write a short summary to /workspace/outputs/summary.md."
        ),
    ) as stream:
        for event in stream:
            if event.type == "agent.session.turn.output_text.delta":
                print(event.delta, end="", flush=True)
    print()

    print("workspace files under /workspace/outputs:")
    page = client.beta.agents.environments.files.list(env_id, path="/workspace/outputs", limit=100)
    while True:
        for workspace_file in page.data:
            print(f"  {workspace_file.path} ({workspace_file.size_bytes} bytes)")
        if page.next is None:
            break
        page = client.beta.agents.environments.files.list(
            env_id,
            path="/workspace/outputs",
            limit=100,
            page=page.next,
        )

    print("artifacts:")
    for artifact in client.beta.agents.sessions.artifacts.list(session.id):
        content = client.beta.agents.sessions.artifacts.content(artifact.id, session_id=session.id)
        target = os.path.basename(artifact.path)
        content.write_to_file(target)
        print(f"  {artifact.path} -> {target}")


if __name__ == "__main__":
    main()
