#!/usr/bin/env python3
"""Verify lockstep versions across Go modules, npm packages, Taskfile, and CI workflows."""

from __future__ import annotations

import argparse
import json
from pathlib import Path
import re
import sys

TARGET_GO_MAJOR_MINOR = "1.26"
TARGET_WAILS_VERSION = "3.1.0"
TARGET_NODE_VERSION = "22"
TARGET_PYTHON_VERSION = "3.13"


def parse_go_mod(text: str) -> dict[str, str]:
    """Extract Go version and Wails v3 version from go.mod."""
    info: dict[str, str] = {}
    go_match = re.search(r"^\s*go\s+([0-9]+\.[0-9]+(?:\.[0-9]+)?)\s*$", text, re.MULTILINE)
    if go_match:
        info["go"] = go_match.group(1)
    wails_match = re.search(
        r"^\s*(?:require\s+)?github\.com/wailsapp/wails/v3\s+v?([^\s]+)", text, re.MULTILINE
    )
    if wails_match:
        info["wails_v3"] = wails_match.group(1)
    return info


def normalize_semver(raw: str) -> str:
    """Strip npm range prefixes (^, ~, =) and leading 'v'."""
    return raw.strip().lstrip("^~=v")


def major_minor(raw: str) -> str:
    """Return 'major.minor'."""
    cleaned = normalize_semver(raw).split("-")[0].split("+")[0]
    parts = cleaned.split(".")
    if len(parts) >= 2:
        return f"{parts[0]}.{parts[1]}"
    return cleaned


def collect_workflow_pins(
    workflows_dir: Path,
) -> dict[str, list[tuple[str, str]]]:
    """Collect (file:line, version) tuples for dependencies in workflows."""
    pins: dict[str, list[tuple[str, str]]] = {
        "go": [],
        "wails": [],
        "node": [],
        "python": [],
    }
    if not workflows_dir.exists():
        return pins

    for wf in sorted(workflows_dir.glob("*.y*ml")):
        lines = wf.read_text(encoding="utf-8").splitlines()
        for idx, line in enumerate(lines, start=1):
            if line.strip().startswith("#"):
                continue
            loc = f"{wf.name}:{idx}"
            # go-version
            gm = re.search(r"go-version:\s*['\"]?([0-9]+\.[0-9]+(?:\.[0-9]+)?)['\"]?", line)
            if gm:
                pins["go"].append((loc, gm.group(1)))
            # wails3 CLI
            wm = re.search(r"github\.com/wailsapp/wails/v3/cmd/wails3@v?([^\s'\"]+)", line)
            if wm:
                pins["wails"].append((loc, wm.group(1)))
            # node-version
            nm = re.search(r"node-version:\s*['\"]?([0-9]+(?:\.[0-9]+)?)['\"]?", line)
            if nm:
                pins["node"].append((loc, nm.group(1)))
            # python-version
            pm = re.search(r"python-version:\s*['\"]?([0-9]+\.[0-9]+)['\"]?", line)
            if pm:
                pins["python"].append((loc, pm.group(1)))

    return pins


def check_lockstep(repo_root: Path) -> list[str]:
    """Return list of human-readable violation strings; empty means compliant."""
    errors: list[str] = []

    go_mod_path = repo_root / "go.mod"
    pkg_json_path = repo_root / "frontend" / "package.json"
    workflows_dir = repo_root / ".github" / "workflows"
    taskfile_path = repo_root / "Taskfile.yml"

    # 1. Check Taskfile.yml exists
    if not taskfile_path.exists():
        errors.append(f"Taskfile.yml missing at {taskfile_path}")

    # 2. Check go.mod
    if not go_mod_path.exists():
        errors.append(f"go.mod missing at {go_mod_path}")
        return errors

    go_mod_info = parse_go_mod(go_mod_path.read_text(encoding="utf-8"))
    go_version = go_mod_info.get("go")
    if not go_version:
        errors.append("go.mod missing 'go' directive")
    elif major_minor(go_version) != TARGET_GO_MAJOR_MINOR:
        errors.append(
            f"go.mod Go version mismatch: expected {TARGET_GO_MAJOR_MINOR}.x, found {go_version}"
        )

    wails_go_ver = go_mod_info.get("wails_v3")
    if not wails_go_ver:
        errors.append("go.mod missing github.com/wailsapp/wails/v3 dependency")
    elif normalize_semver(wails_go_ver) != TARGET_WAILS_VERSION:
        errors.append(
            f"go.mod Wails v3 version mismatch: expected {TARGET_WAILS_VERSION}, found {wails_go_ver}"
        )

    # 3. Check frontend/package.json
    if not pkg_json_path.exists():
        errors.append(f"frontend/package.json missing at {pkg_json_path}")
    else:
        try:
            pkg_data = json.loads(pkg_json_path.read_text(encoding="utf-8"))
            deps = {**pkg_data.get("dependencies", {}), **pkg_data.get("devDependencies", {})}
            wails_runtime = deps.get("@wailsio/runtime")
            if not wails_runtime:
                errors.append("frontend/package.json missing @wailsio/runtime dependency")
            else:
                norm_npm = normalize_semver(wails_runtime)
                if norm_npm != TARGET_WAILS_VERSION:
                    errors.append(
                        f"frontend/package.json @wailsio/runtime mismatch: expected {TARGET_WAILS_VERSION}, found {norm_npm}"
                    )
        except Exception as e:
            errors.append(f"Could not parse frontend/package.json: {e}")

    # 4. Check CI workflows
    pins = collect_workflow_pins(workflows_dir)

    for loc, pin in pins["go"]:
        if major_minor(pin) != TARGET_GO_MAJOR_MINOR:
            errors.append(
                f"Workflow go-version mismatch at .github/workflows/{loc}: expected {TARGET_GO_MAJOR_MINOR}, found {pin}"
            )

    for loc, pin in pins["wails"]:
        if normalize_semver(pin) != TARGET_WAILS_VERSION:
            errors.append(
                f"Workflow wails3 CLI pin mismatch at .github/workflows/{loc}: expected {TARGET_WAILS_VERSION}, found {pin}"
            )

    for loc, pin in pins["node"]:
        if pin.split(".")[0] != TARGET_NODE_VERSION:
            errors.append(
                f"Workflow node-version mismatch at .github/workflows/{loc}: expected {TARGET_NODE_VERSION}, found {pin}"
            )

    for loc, pin in pins["python"]:
        if pin != TARGET_PYTHON_VERSION:
            errors.append(
                f"Workflow python-version mismatch at .github/workflows/{loc}: expected {TARGET_PYTHON_VERSION}, found {pin}"
            )

    return errors


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--repo-root",
        type=Path,
        default=Path(__file__).resolve().parent.parent,
        help="Repository root (defaults to script's parent's parent)",
    )
    args = parser.parse_args()

    errors = check_lockstep(args.repo_root)
    if errors:
        print(f"FAIL: {len(errors)} lockstep violation(s):", file=sys.stderr)
        for err in errors:
            print(f"  - {err}", file=sys.stderr)
        return 1

    print("OK: All dependency versions (Go 1.26, Wails v3.1.0, Node 22, Python 3.13) are in lockstep.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
