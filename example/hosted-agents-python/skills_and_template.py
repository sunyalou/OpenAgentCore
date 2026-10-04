#!/usr/bin/env python3
"""Upload a Skill, create an Environment Template and start a Session from it.

The skill is generated in a temporary directory, so the script needs no local
skill checkout. The Session freezes the template when it starts.
"""

from __future__ import annotations

import os
import sys
import tempfile

from openai import OpenAI

REQUIRED_ENV = ("OPENAI_BASE_URL", "OPENAI_API_KEY", "OAC_MODEL")

SKILL_MD = """# Greeting skill

When the user asks for a greeting, reply with one short friendly sentence.
"""


def main() -> None:
    for name in REQUIRED_ENV:
        if not os.environ.get(name):
            sys.exit(f"Set {name}; see example/hosted-agents-python/README.md.")

    client = OpenAI()

    with tempfile.TemporaryDirectory() as skill_dir:
        os.makedirs(os.path.join(skill_dir, "greeting"))
        skill_path = os.path.join(skill_dir, "greeting", "SKILL.md")
        with open(skill_path, "w", encoding="utf-8") as skill_file:
            skill_file.write(SKILL_MD)

        with open(skill_path, "rb") as skill_file:
            skill = client.skills.create(files=[("greeting/SKILL.md", skill_file)])
        with open(skill_path, "rb") as skill_file:
            client.skills.versions.create(
                skill.id,
                files=[("greeting/SKILL.md", skill_file)],
                default=True,
            )
    print(f"skill: {skill.id}")

    template = client.beta.agents.environments.templates.create(
        name="python-data",
        setup_commands=[{"command": "mkdir -p /workspace/outputs"}],
        skills=[{"type": "skill_reference", "skill_id": skill.id}],
    )
    print(f"template: {template.id}")

    agent = client.beta.agents.create(
        model=os.environ["OAC_MODEL"],
        name="Skill demo",
        instructions="Use the attached greeting skill when asked to greet.",
    )
    session = client.beta.agents.sessions.create(
        environment={"type": "openai_hosted", "environment_template_id": template.id},
        agent_id=agent.id,
    )
    print(f"session: {session.id}")

    print("assistant: ", end="", flush=True)
    with client.beta.agents.sessions.stream(
        session.id,
        input="Greet me in one sentence.",
    ) as stream:
        for event in stream:
            if event.type == "agent.session.turn.output_text.delta":
                print(event.delta, end="", flush=True)
    print()


if __name__ == "__main__":
    main()
