"""Unit tests for scripts/check-okf-hygiene.py."""

from __future__ import annotations

import importlib.util
from pathlib import Path
import tempfile
import unittest

SCRIPT_PATH = Path(__file__).resolve().parents[1] / "check-okf-hygiene.py"
SPEC = importlib.util.spec_from_file_location("check_okf_hygiene", SCRIPT_PATH)
assert SPEC is not None and SPEC.loader is not None
HYGIENE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(HYGIENE)

REPO_ROOT = Path(__file__).resolve().parents[2]


class TestOKFHygiene(unittest.TestCase):
    def test_repo_passes_hygiene(self) -> None:
        self.assertEqual(HYGIENE.check_hygiene(REPO_ROOT), [])

    def test_disallowed_markdown_detected(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self._seed_minimal_repo(root)
            (root / "backend").mkdir()
            (root / "backend" / "TODO.md").write_text("# stray\n", encoding="utf-8")
            errors = HYGIENE.check_hygiene(root)
            self.assertTrue(any("backend/TODO.md" in e for e in errors))

    def test_unindexed_okf_doc_detected(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self._seed_minimal_repo(root)
            doc_dir = root / "okf" / "02_present_state"
            doc_dir.mkdir(parents=True)
            (doc_dir / "extra.md").write_text("# Extra\n", encoding="utf-8")
            errors = HYGIENE.check_hygiene(root)
            self.assertTrue(any("okf/02_present_state/extra.md" in e for e in errors))

    def test_broken_link_detected(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self._seed_minimal_repo(root)
            (root / "okf" / "00_index.md").write_text(
                "[missing](./02_present_state/missing.md)\n", encoding="utf-8"
            )
            errors = HYGIENE.check_hygiene(root)
            self.assertTrue(any("Broken relative link" in e for e in errors))

    def _seed_minimal_repo(self, root: Path) -> None:
        (root / "AGENTS.md").write_text("# Root\n", encoding="utf-8")
        (root / ".github").mkdir()
        (root / ".github" / "copilot-instructions.md").write_text("# Copilot\n", encoding="utf-8")
        (root / "okf").mkdir()
        (root / "okf" / "AGENTS.md").write_text("# OKF\n", encoding="utf-8")
        (root / "okf" / "00_index.md").write_text("# Index\n", encoding="utf-8")


if __name__ == "__main__":
    unittest.main()
