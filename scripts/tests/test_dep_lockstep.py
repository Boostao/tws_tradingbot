"""Unit tests for scripts/check-dep-lockstep.py."""

from __future__ import annotations

import importlib.util
from pathlib import Path
import tempfile
import unittest

SCRIPT_PATH = Path(__file__).resolve().parents[1] / "check-dep-lockstep.py"
SPEC = importlib.util.spec_from_file_location("check_dep_lockstep", SCRIPT_PATH)
assert SPEC is not None and SPEC.loader is not None
LOCKSTEP = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(LOCKSTEP)

REPO_ROOT = Path(__file__).resolve().parents[2]


class TestDepLockstep(unittest.TestCase):
    def _seed_valid_repo(self, root: Path) -> None:
        (root / "Taskfile.yml").write_text("version: '3'\n", encoding="utf-8")
        (root / "go.mod").write_text(
            "module tws_traderbot\n\ngo 1.26.0\n\nrequire github.com/wailsapp/wails/v3 v3.1.0\n",
            encoding="utf-8",
        )
        fe = root / "frontend"
        fe.mkdir(parents=True)
        (fe / "package.json").write_text(
            '{"dependencies": {"@wailsio/runtime": "^3.1.0"}}\n',
            encoding="utf-8",
        )
        wf = root / ".github" / "workflows"
        wf.mkdir(parents=True)
        (wf / "ci.yml").write_text(
            """name: CI
jobs:
  test:
    steps:
      - uses: actions/setup-go@v5
        with:
          go-version: '1.26'
      - uses: actions/setup-node@v4
        with:
          node-version: '22'
      - uses: actions/setup-python@v5
        with:
          python-version: '3.13'
      - run: go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0
""",
            encoding="utf-8",
        )

    def test_valid_synthetic_repo_passes(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self._seed_valid_repo(root)
            errors = LOCKSTEP.check_lockstep(root)
            self.assertEqual(errors, [])

    def test_missing_go_mod(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self._seed_valid_repo(root)
            (root / "go.mod").unlink()
            errors = LOCKSTEP.check_lockstep(root)
            self.assertTrue(any("go.mod missing" in e for e in errors))

    def test_mismatched_go_version(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self._seed_valid_repo(root)
            (root / "go.mod").write_text(
                "module tws_traderbot\n\ngo 1.24.0\n\nrequire github.com/wailsapp/wails/v3 v3.1.0\n",
                encoding="utf-8",
            )
            errors = LOCKSTEP.check_lockstep(root)
            self.assertTrue(any("Go version mismatch" in e for e in errors))

    def test_mismatched_wails_npm_runtime(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self._seed_valid_repo(root)
            (root / "frontend" / "package.json").write_text(
                '{"dependencies": {"@wailsio/runtime": "^3.0.0"}}\n',
                encoding="utf-8",
            )
            errors = LOCKSTEP.check_lockstep(root)
            self.assertTrue(any("@wailsio/runtime mismatch" in e for e in errors))

    def test_workflow_go_version_mismatch(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self._seed_valid_repo(root)
            (root / ".github" / "workflows" / "ci.yml").write_text(
                """name: CI
jobs:
  test:
    steps:
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
""",
                encoding="utf-8",
            )
            errors = LOCKSTEP.check_lockstep(root)
            self.assertTrue(any("Workflow go-version mismatch" in e for e in errors))

    def test_workflow_wails_cli_mismatch(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self._seed_valid_repo(root)
            (root / ".github" / "workflows" / "ci.yml").write_text(
                """name: CI
jobs:
  test:
    steps:
      - run: go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0
""",
                encoding="utf-8",
            )
            errors = LOCKSTEP.check_lockstep(root)
            self.assertTrue(any("Workflow wails3 CLI pin mismatch" in e for e in errors))

    def test_workflow_node_version_mismatch(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self._seed_valid_repo(root)
            (root / ".github" / "workflows" / "ci.yml").write_text(
                """name: CI
jobs:
  test:
    steps:
      - uses: actions/setup-node@v4
        with:
          node-version: '20'
""",
                encoding="utf-8",
            )
            errors = LOCKSTEP.check_lockstep(root)
            self.assertTrue(any("Workflow node-version mismatch" in e for e in errors))

    def test_workflow_python_version_mismatch(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self._seed_valid_repo(root)
            (root / ".github" / "workflows" / "ci.yml").write_text(
                """name: CI
jobs:
  test:
    steps:
      - uses: actions/setup-python@v5
        with:
          python-version: '3.11'
""",
                encoding="utf-8",
            )
            errors = LOCKSTEP.check_lockstep(root)
            self.assertTrue(any("Workflow python-version mismatch" in e for e in errors))

    def test_missing_taskfile(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self._seed_valid_repo(root)
            (root / "Taskfile.yml").unlink()
            errors = LOCKSTEP.check_lockstep(root)
            self.assertTrue(any("Taskfile.yml missing" in e for e in errors))

    def test_malformed_package_json(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self._seed_valid_repo(root)
            (root / "frontend" / "package.json").write_text("{ invalid json }", encoding="utf-8")
            errors = LOCKSTEP.check_lockstep(root)
            self.assertTrue(any("Could not parse frontend/package.json" in e for e in errors))

    def test_commented_workflow_pin_ignored(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self._seed_valid_repo(root)
            (root / ".github" / "workflows" / "ci.yml").write_text(
                """name: CI
jobs:
  test:
    steps:
      # go-version: '1.24'
      - uses: actions/setup-go@v5
        with:
          go-version: '1.26'
""",
                encoding="utf-8",
            )
            errors = LOCKSTEP.check_lockstep(root)
            self.assertEqual(errors, [])


if __name__ == "__main__":
    unittest.main()
