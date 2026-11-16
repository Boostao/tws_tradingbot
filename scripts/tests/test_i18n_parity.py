"""Unit tests for scripts/check-i18n-parity.py."""

from __future__ import annotations

import importlib.util
from pathlib import Path
import tempfile
import unittest

SCRIPT_PATH = Path(__file__).resolve().parents[1] / "check-i18n-parity.py"
SPEC = importlib.util.spec_from_file_location("check_i18n_parity", SCRIPT_PATH)
assert SPEC is not None and SPEC.loader is not None
PARITY = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(PARITY)

REPO_ROOT = Path(__file__).resolve().parents[2]


class TestI18nParity(unittest.TestCase):
    def _seed_valid_repo(self, root: Path) -> None:
        trans_dir = root / "frontend" / "src" / "lib" / "i18n"
        trans_dir.mkdir(parents=True)
        (trans_dir / "translations.ts").write_text(
            """export const translations = {
  en: {
    welcome: "Welcome",
    login: "Log in",
  },
  fr: {
    welcome: "Bienvenue",
    login: "Connexion",
  },
};
""",
            encoding="utf-8",
        )
        comp_dir = root / "frontend" / "src" / "components"
        comp_dir.mkdir(parents=True)
        (comp_dir / "Header.svelte").write_text(
            """<script>
  import { t } from '$lib/i18n';
</script>
<h1>{t('welcome')}</h1>
<button>{t('login')}</button>
""",
            encoding="utf-8",
        )

    def test_repo_passes_i18n_parity(self) -> None:
        errors = PARITY.check_i18n_parity(REPO_ROOT)
        self.assertEqual(errors, [])

    def test_valid_synthetic_repo_passes(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self._seed_valid_repo(root)
            errors = PARITY.check_i18n_parity(root)
            self.assertEqual(errors, [])

    def test_missing_translations_file(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self._seed_valid_repo(root)
            (root / "frontend" / "src" / "lib" / "i18n" / "translations.ts").unlink()
            errors = PARITY.check_i18n_parity(root)
            self.assertTrue(any("Translations file missing" in e for e in errors))

    def test_asymmetric_en_key(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self._seed_valid_repo(root)
            (root / "frontend" / "src" / "lib" / "i18n" / "translations.ts").write_text(
                """export const translations = {
  en: {
    welcome: "Welcome",
    extra_en: "Only in EN",
  },
  fr: {
    welcome: "Bienvenue",
  },
};
""",
                encoding="utf-8",
            )
            errors = PARITY.check_i18n_parity(root)
            self.assertTrue(any("missing in FR" in e for e in errors))

    def test_asymmetric_fr_key(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self._seed_valid_repo(root)
            (root / "frontend" / "src" / "lib" / "i18n" / "translations.ts").write_text(
                """export const translations = {
  en: {
    welcome: "Welcome",
  },
  fr: {
    welcome: "Bienvenue",
    extra_fr: "Seulement en FR",
  },
};
""",
                encoding="utf-8",
            )
            errors = PARITY.check_i18n_parity(root)
            self.assertTrue(any("missing in EN" in e for e in errors))

    def test_prohibited_jargon_detected(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self._seed_valid_repo(root)
            (root / "frontend" / "src" / "lib" / "i18n" / "translations.ts").write_text(
                """export const translations = {
  en: {
    welcome: "Powered by FastAPI backend",
  },
  fr: {
    welcome: "Propulsé par le backend FastAPI",
  },
};
""",
                encoding="utf-8",
            )
            errors = PARITY.check_i18n_parity(root)
            self.assertTrue(any("Prohibited jargon 'fastapi'" in e for e in errors))

    def test_missing_code_key_detected(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self._seed_valid_repo(root)
            (root / "frontend" / "src" / "components" / "NewView.svelte").write_text(
                """<script>
  import { t } from '$lib/i18n';
</script>
<p>{t('unregistered_code_key')}</p>
""",
                encoding="utf-8",
            )
            errors = PARITY.check_i18n_parity(root)
            self.assertTrue(any("unregistered_code_key" in e for e in errors))

    def test_missing_parameterized_code_key_detected(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self._seed_valid_repo(root)
            (root / "frontend" / "src" / "components" / "ParamView.svelte").write_text(
                """<script>
  import { t } from '$lib/i18n';
</script>
<p>{t('unregistered_param_key', { count: 1 })}</p>
""",
                encoding="utf-8",
            )
            errors = PARITY.check_i18n_parity(root)
            self.assertTrue(any("unregistered_param_key" in e for e in errors))

    def test_missing_special_character_code_key_detected(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self._seed_valid_repo(root)
            (root / "frontend" / "src" / "components" / "SpecialView.svelte").write_text(
                """<script>
  import { t } from '$lib/i18n';
</script>
<p>{t('greater than (>)' )}</p>
""",
                encoding="utf-8",
            )
            errors = PARITY.check_i18n_parity(root)
            self.assertTrue(any("greater than (>)" in e for e in errors))


if __name__ == "__main__":
    unittest.main()
