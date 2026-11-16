#!/usr/bin/env python3
"""Check i18n parity between EN and FR in frontend/src/lib/i18n/translations.ts and scan code usage."""

from __future__ import annotations

import argparse
from pathlib import Path
import re
import sys


def extract_translation_keys(part: str) -> set[str]:
    """Extract object keys from a TypeScript object literal block."""
    pattern = r"^\s*(?:'([^']+)'|\"([^\"]+)\"|([a-zA-Z0-9_]+))\s*:"
    keys = set()
    for m in re.finditer(pattern, part, re.MULTILINE):
        k = m.group(1) or m.group(2) or m.group(3)
        if k not in ("en", "fr"):
            keys.add(k)
    return keys


def extract_code_usage_keys(src_dir: Path) -> set[str]:
    """Extract keys used in t('...') or $t('...') calls across frontend src files."""
    call_pattern = re.compile(r"(?:\$t|\bt)\(\s*['\"]([^'\"]+)['\"]\s*[,)]")
    used_keys = set()
    if not src_dir.exists():
        return used_keys

    for file_path in src_dir.rglob("*"):
        if file_path.suffix in (".svelte", ".ts") and not file_path.name.endswith("translations.ts"):
            try:
                text = file_path.read_text(encoding="utf-8")
                for match in call_pattern.finditer(text):
                    used_keys.add(match.group(1))
            except Exception:
                pass
    return used_keys


def check_i18n_parity(repo_root: Path) -> list[str]:
    """Perform full i18n parity check: EN/FR symmetry, zero jargon, and code usage coverage."""
    errors: list[str] = []
    translations_path = repo_root / "frontend" / "src" / "lib" / "i18n" / "translations.ts"
    if not translations_path.exists():
        return [f"Translations file missing: {translations_path}"]

    content = translations_path.read_text(encoding="utf-8")
    en_idx = content.find("en: {")
    fr_idx = content.find("fr: {")
    if en_idx == -1 or fr_idx == -1:
        return ["Could not find en: { or fr: { blocks in translations.ts"]

    en_part = content[en_idx:fr_idx]
    fr_part = content[fr_idx:]

    en_keys = extract_translation_keys(en_part)
    fr_keys = extract_translation_keys(fr_part)

    # 1. Parity between English and French keys
    missing_in_fr = en_keys - fr_keys
    if missing_in_fr:
        errors.append(f"Keys present in EN but missing in FR ({len(missing_in_fr)}): {sorted(missing_in_fr)}")

    missing_in_en = fr_keys - en_keys
    if missing_in_en:
        errors.append(f"Keys present in FR but missing in EN ({len(missing_in_en)}): {sorted(missing_in_en)}")

    # 2. Check for prohibited jargon
    banned_terms = ["nautilus", "fastapi", "pydantic"]
    lower_content = content.lower()
    for term in banned_terms:
        if term in lower_content:
            errors.append(f"Prohibited jargon '{term}' found in translations.ts")

    # 3. Check code usage against defined keys
    src_dir = repo_root / "frontend" / "src"
    used_keys = extract_code_usage_keys(src_dir)
    missing_from_translations = used_keys - en_keys
    if missing_from_translations:
        errors.append(
            f"Keys used in frontend code but missing in translations.ts ({len(missing_from_translations)}): "
            f"{sorted(missing_from_translations)}"
        )

    return errors


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--repo-root",
        type=Path,
        default=Path(__file__).resolve().parent.parent,
        help="Repository root directory",
    )
    args = parser.parse_args()

    errors = check_i18n_parity(args.repo_root)
    if errors:
        print(f"FAIL: {len(errors)} i18n violation(s):", file=sys.stderr)
        for err in errors:
            print(f"  - {err}", file=sys.stderr)
        return 1

    print("OK: 100% translation key parity verified (symmetric, zero prohibited jargon, all UI calls covered).")
    return 0


if __name__ == "__main__":
    sys.exit(main())
