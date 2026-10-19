#!/usr/bin/env python3
"""Create a self-contained disposable repository topology for daemon tests."""

from __future__ import annotations

import argparse
from dataclasses import dataclass
import json
from pathlib import Path
import shutil
import subprocess
import tempfile


FIXTURE_ROOT = Path(__file__).resolve().parent / "fixtures" / "public-repository"
REQUIRED_PUBLIC_FILES = (
    "schemas/post.v1.schema.json",
    "schemas/latest-feed.v1.schema.json",
    "feed/latest.json",
    "feed/catalog.json",
    "feed/stats.json",
    "feed/relations.json",
)


@dataclass(frozen=True)
class Topology:
    root: Path
    private: Path
    public: Path
    daemon: Path
    public_remote: Path

    def summary(self) -> dict[str, str]:
        return {
            "root": str(self.root),
            "private": str(self.private),
            "public": str(self.public),
            "daemon": str(self.daemon),
            "public_remote": str(self.public_remote),
            "public_head": git(self.public, "rev-parse", "HEAD").strip(),
            "remote_head": git(self.public_remote, "rev-parse", "refs/heads/main").strip(),
        }


def git(repository: Path, *arguments: str) -> str:
    process = subprocess.run(
        ["git", "-C", str(repository), *arguments],
        check=False,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    if process.returncode:
        detail = process.stderr.decode("utf-8", "replace").strip()
        raise RuntimeError(f"git {' '.join(arguments)} failed in {repository}: {detail}")
    return process.stdout.decode("utf-8", "replace")


def initialize_repository(destination: Path, *, public: bool = False) -> None:
    subprocess.run(
        ["git", "init", "--initial-branch=main", str(destination)],
        check=True,
        stdout=subprocess.DEVNULL,
    )
    if public:
        for relative in REQUIRED_PUBLIC_FILES:
            source = FIXTURE_ROOT / relative
            target = destination / relative
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(source, target)
    else:
        (destination / ".test-fixture").write_text("isolated temporary repository\n")
    git(destination, "config", "user.name", "The Machine Writes Harness")
    git(destination, "config", "user.email", "harness@themachinewrites.invalid")
    git(destination, "add", "--all")
    git(destination, "commit", "-m", "fixture: initialize temporary repository")


def require_empty_initial_state(public: Path) -> None:
    for relative in REQUIRED_PUBLIC_FILES:
        if not (public / relative).is_file():
            raise RuntimeError(f"public fixture is missing {relative}")
    latest = json.loads((public / "feed/latest.json").read_text())
    if latest.get("schema_version") != 1 or latest.get("posts") != []:
        raise RuntimeError("latest feed fixture must be an empty schema v1 record")
    catalog = json.loads((public / "feed/catalog.json").read_text())
    if catalog.get("schema_version") != 1 or catalog.get("months") != []:
        raise RuntimeError("catalog fixture must be empty")


def create_topology(root: Path | None = None) -> Topology:
    parent = Path(root) if root is not None else Path(tempfile.mkdtemp(prefix="tmw-daemon-test-"))
    parent.mkdir(parents=True, exist_ok=True)
    if any(parent.iterdir()):
        raise RuntimeError("harness root must be empty")

    private = parent / "themachinewrites-private"
    public = parent / "themachinewrites-public"
    daemon = parent / "themachinewrites-daemon"
    remote = parent / "remotes" / "themachinewrites-public.git"
    remote.parent.mkdir(parents=True)
    subprocess.run(
        ["git", "init", "--bare", "--initial-branch=main", str(remote)],
        check=True,
        stdout=subprocess.DEVNULL,
    )

    initialize_repository(public, public=True)
    require_empty_initial_state(public)
    git(public, "remote", "add", "origin", str(remote.resolve()))
    git(public, "push", "-u", "origin", "main")
    initialize_repository(private)
    initialize_repository(daemon)

    return Topology(parent.resolve(), private.resolve(), public.resolve(), daemon.resolve(), remote.resolve())


def verify_topology(topology: Topology) -> None:
    roots = [git(path, "rev-parse", "--show-toplevel").strip() for path in (topology.private, topology.public, topology.daemon)]
    if len(set(roots)) != 3 or len({(Path(path) / ".git").resolve() for path in roots}) != 3:
        raise RuntimeError(f"repositories do not have three distinct Git roots: {roots}")
    for path in (topology.private, topology.daemon):
        remotes = git(path, "remote").strip()
        if remotes:
            raise RuntimeError(f"private fixture gained a remote: {path}: {remotes}")
    public_head = git(topology.public, "rev-parse", "HEAD").strip()
    remote_head = git(topology.public_remote, "rev-parse", "refs/heads/main").strip()
    if not public_head or public_head != remote_head:
        raise RuntimeError("bare public remote does not match the fixture public commit")
    if git(topology.public, "remote", "get-url", "origin").strip() != str(topology.public_remote):
        raise RuntimeError("public origin does not point at the temporary bare remote")
    require_empty_initial_state(topology.public)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, help="empty directory for the disposable topology")
    arguments = parser.parse_args()
    topology = create_topology(arguments.root)
    verify_topology(topology)
    print(json.dumps(topology.summary(), indent=2, sort_keys=True))


if __name__ == "__main__":
    main()
