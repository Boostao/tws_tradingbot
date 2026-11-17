"""Opaque-box tests for Cobalt V2 packaging, Wayland AppID, brand assets, site/dist, and OKF/i18n/bindings gates (R6 & R9)."""

from __future__ import annotations

import configparser
import importlib.util
import json
from pathlib import Path
import re
import unittest
import xml.etree.ElementTree as ET

from PIL import Image, ImageStat

REPO_ROOT = Path(__file__).resolve().parents[2]
APP_ID = "ca.boostao.tws_traderbot"
GTK_APP_ID_RE = re.compile(r"^[A-Za-z_][A-Za-z0-9_-]*(\.[A-Za-z_][A-Za-z0-9_-]*)+$")

COBALT_PALETTE_HEX = (
    "#263c8a",
    "#1d63ed",
    "#38bdf8",
    "#0e0c28",
    "#05070b",
    "#0b111e",
    "#faa031",
    "#edc44e",
)


def _load_script_module(name: str, filename: str):
    path = REPO_ROOT / "scripts" / filename
    spec = importlib.util.spec_from_file_location(name, path)
    assert spec is not None and spec.loader is not None
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


def _is_valid_gtk_app_id(app_id: str) -> bool:
    return bool(app_id) and len(app_id) <= 255 and bool(GTK_APP_ID_RE.match(app_id))


def _svg_primitive_summary(svg_path: Path) -> tuple[str, int, int, int]:
    """Parse an SVG and return (viewBox, polygon_count, path_count, rect_count)."""
    tree = ET.parse(svg_path)
    root = tree.getroot()
    tag = root.tag.split("}")[-1].lower()
    if tag != "svg":
        raise AssertionError(f"{svg_path} root element is <{tag}>, expected <svg>")
    view_box = root.attrib.get("viewBox", "").strip()
    polygons = 0
    paths = 0
    rects = 0
    for elem in root.iter():
        local = elem.tag.split("}")[-1].lower()
        if local == "polygon":
            polygons += 1
        elif local == "path":
            paths += 1
        elif local == "rect":
            rects += 1
    return view_box, polygons, paths, rects


class TestV2PackagingAndBrand(unittest.TestCase):
    # ------------------------------------------------------------------
    # PKG-01: main.go Wayland ApplicationID + Icon embed matching config
    # ------------------------------------------------------------------
    def test_pkg_01_main_go_wayland_app_id_and_icon_embed(self) -> None:
        config_yml = (REPO_ROOT / "build" / "config.yml").read_text(encoding="utf-8")
        m_id = re.search(r'productIdentifier:\s*["\']([^"\']+)["\']', config_yml)
        self.assertIsNotNone(m_id, "build/config.yml must declare productIdentifier")
        product_id = m_id.group(1)
        self.assertEqual(product_id, APP_ID)
        self.assertTrue(
            _is_valid_gtk_app_id(product_id),
            f"productIdentifier {product_id!r} violates GTK g_application_id_is_valid",
        )
        # Boundary verification on helper
        self.assertFalse(_is_valid_gtk_app_id("invalid_single_segment"))
        self.assertFalse(_is_valid_gtk_app_id("1starts.with.digit"))

        main_go = (REPO_ROOT / "main.go").read_text(encoding="utf-8")
        self.assertRegex(
            main_go,
            r"//go:embed\s+build/appicon\.png",
            "main.go must embed build/appicon.png for window/Wayland icon",
        )
        self.assertRegex(
            main_go,
            r"\bIcon\s*:",
            "main.go application.Options must pass Icon:",
        )
        # Accept either literal string or Go constant assigned "ca.boostao.tws_traderbot"
        literal_match = re.search(
            r'Linux\s*:\s*application\.LinuxOptions\s*\{[^}]*ApplicationID\s*:\s*"ca\.boostao\.tws_traderbot"',
            main_go,
        )
        const_match = re.search(
            r'Linux\s*:\s*application\.LinuxOptions\s*\{[^}]*ApplicationID\s*:\s*([A-Za-z_][A-Za-z0-9_]*)',
            main_go,
        )
        const_ok = False
        if const_match:
            ident = const_match.group(1)
            const_ok = bool(
                re.search(rf'\b{re.escape(ident)}\s*=\s*"ca\.boostao\.tws_traderbot"', main_go)
            )
        self.assertTrue(
            bool(literal_match or const_ok),
            f'main.go application.Options must configure Linux: application.LinuxOptions{{ApplicationID: "{APP_ID}"}}',
        )

    # ------------------------------------------------------------------
    # PKG-02: Linux .desktop manifest
    # ------------------------------------------------------------------
    def test_pkg_02_linux_desktop_manifest(self) -> None:
        desktop_path = REPO_ROOT / "build" / "linux" / f"{APP_ID}.desktop"
        self.assertTrue(desktop_path.is_file(), f"Missing Linux desktop entry: {desktop_path}")

        raw = desktop_path.read_text(encoding="utf-8")
        parser = configparser.ConfigParser(interpolation=None)
        parser.optionxform = str  # preserve case of Desktop Entry keys
        parser.read_string(raw)

        self.assertIn("Desktop Entry", parser.sections(), f"{desktop_path} missing [Desktop Entry]")
        entry = parser["Desktop Entry"]

        self.assertEqual(entry.get("Type"), "Application")
        name = entry.get("Name", "")
        self.assertTrue(
            "cobalt" in name.lower() or "tws_traderbot" in name.lower(),
            f".desktop Name={name!r} should reference Cobalt or tws_traderbot",
        )
        exec_cmd = entry.get("Exec", "")
        self.assertIn("tws_traderbot", exec_cmd, f".desktop Exec={exec_cmd!r} must launch tws_traderbot")
        self.assertEqual(entry.get("Icon"), APP_ID, f".desktop Icon must equal {APP_ID!r}")
        self.assertEqual(
            entry.get("StartupWMClass"),
            APP_ID,
            f".desktop StartupWMClass must equal {APP_ID!r}",
        )
        self.assertEqual(entry.get("Terminal"), "false")
        categories = entry.get("Categories", "")
        self.assertTrue(
            categories.endswith(";") and len(categories) > 3,
            f".desktop Categories={categories!r} must be a valid semicolon-terminated list",
        )

    # ------------------------------------------------------------------
    # PKG-03: Taskfile.yml desktop & hicolor icon install in dev / linux:build
    # ------------------------------------------------------------------
    def test_pkg_03_taskfile_installs_desktop_and_hicolor_icon(self) -> None:
        taskfile = (REPO_ROOT / "Taskfile.yml").read_text(encoding="utf-8")

        # Resolve Taskfile top-level vars (e.g. {{.APP_ID}} -> ca.boostao.tws_traderbot)
        for var_name, var_val in re.findall(r'(?m)^  ([A-Z0-9_]+):\s*"?([^"\n]+)"?\s*$', taskfile):
            taskfile = taskfile.replace(f"{{{{.{var_name}}}}}", var_val.strip())

        # Extract top-level task blocks under `tasks:`
        tasks_idx = taskfile.find("\ntasks:")
        if tasks_idx == -1 and taskfile.startswith("tasks:"):
            tasks_idx = 0
        self.assertNotEqual(tasks_idx, -1, "Taskfile.yml missing tasks: section")
        tasks_section = taskfile[tasks_idx:]

        block_matches = list(re.finditer(r"(?m)^  ([a-zA-Z0-9_:-]+):\s*$", tasks_section))
        blocks: dict[str, str] = {}
        for idx, match in enumerate(block_matches):
            name = match.group(1)
            start = match.end()
            end = block_matches[idx + 1].start() if idx + 1 < len(block_matches) else len(tasks_section)
            blocks[name] = tasks_section[start:end]

        self.assertIn("linux:build", blocks, "Taskfile.yml missing linux:build")
        self.assertIn("dev", blocks, "Taskfile.yml missing dev")

        def task_or_deps_contain(task_name: str, visited: set[str] | None = None) -> str:
            if visited is None:
                visited = set()
            if task_name in visited or task_name not in blocks:
                return ""
            visited.add(task_name)
            body = blocks[task_name]
            combined = [body]
            for dep in re.findall(r"\btask:\s*['\"]?([a-zA-Z0-9_:-]+)['\"]?", body):
                combined.append(task_or_deps_contain(dep, visited))
            return "\n".join(combined)

        for root_task in ("linux:build", "dev"):
            resolved = task_or_deps_contain(root_task)
            has_desktop = (
                f"{APP_ID}.desktop" in resolved and "applications" in resolved
            )
            has_hicolor = (
                "hicolor" in resolved and APP_ID in resolved and ("appicon.png" in resolved or "icons" in resolved)
            )
            # Also accept `build:dev` in build/config.yml calling Taskfile install task if `dev` invokes wails3 dev
            if root_task == "dev" and "build:dev" in blocks and not (has_desktop and has_hicolor):
                resolved_dev = resolved + "\n" + task_or_deps_contain("build:dev")
                has_desktop = f"{APP_ID}.desktop" in resolved_dev and "applications" in resolved_dev
                has_hicolor = "hicolor" in resolved_dev and APP_ID in resolved_dev

            self.assertTrue(
                has_desktop,
                f"Taskfile.yml `{root_task}` (or its subtasks) must install {APP_ID}.desktop into applications directory",
            )
            self.assertTrue(
                has_hicolor,
                f"Taskfile.yml `{root_task}` (or its subtasks) must install hicolor icon for {APP_ID}",
            )

    # ------------------------------------------------------------------
    # PKG-04: Raster icon integrity (PNG & ICO)
    # ------------------------------------------------------------------
    def test_pkg_04_raster_icons_png_and_ico_integrity(self) -> None:
        png_path = REPO_ROOT / "build" / "appicon.png"
        self.assertTrue(png_path.is_file(), f"Missing {png_path}")
        with Image.open(png_path) as im:
            self.assertEqual(im.format, "PNG")
            self.assertIn(im.mode, ("RGBA", "RGB"))
            w, h = im.size
            self.assertEqual(w, h, "appicon.png must be square")
            self.assertGreaterEqual(w, 256, "appicon.png must be >= 256x256")
            rgba = im.convert("RGBA")
            stat = ImageStat.Stat(rgba)
            self.assertGreater(max(stat.stddev[:3]), 5.0, "appicon.png appears blank/uniform")

            # Verify Cobalt blue accent pixels (B > R and B >= 120) exist on the emblem
            thumb = rgba.resize((64, 64))
            pixels = thumb.get_flattened_data() if hasattr(thumb, "get_flattened_data") else thumb.getdata()
            blue_pixels = sum(
                1 for (r, g, b, a) in pixels if a > 128 and b > r + 25 and b >= 120
            )
            self.assertGreater(
                blue_pixels,
                20,
                f"appicon.png must feature Cobalt blue emblem pixels (found {blue_pixels})",
            )

        ico_path = REPO_ROOT / "build" / "windows" / "icon.ico"
        self.assertTrue(ico_path.is_file(), f"Missing {ico_path}")
        with Image.open(ico_path) as ico:
            self.assertEqual(ico.format, "ICO")
            sizes = ico.info.get("sizes") or set()
            self.assertIn((256, 256), sizes, f"icon.ico missing 256x256 frame; sizes={sizes}")
            self.assertGreaterEqual(len(sizes), 2, f"icon.ico must be multi-size; sizes={sizes}")

    # ------------------------------------------------------------------
    # PKG-05: SVG brand emblems & zero stock Svelte logo
    # ------------------------------------------------------------------
    def test_pkg_05_cobalt_svg_emblems_and_sidebar_header(self) -> None:
        app_svg = REPO_ROOT / "frontend" / "src" / "lib" / "assets" / "favicon.svg"
        site_svg = REPO_ROOT / "site" / "favicon.svg"
        sidebar_svelte = REPO_ROOT / "frontend" / "src" / "lib" / "components" / "Sidebar.svelte"

        for svg_path in (app_svg, site_svg):
            self.assertTrue(svg_path.is_file(), f"Missing SVG emblem: {svg_path}")
            raw = svg_path.read_text(encoding="utf-8")
            lower = raw.lower()
            self.assertNotIn("svelte-logo", lower, f"{svg_path} still contains stock svelte-logo")
            self.assertNotIn("#ff3e00", lower, f"{svg_path} still contains stock Svelte #ff3e00")

            view_box, polygons, paths, rects = _svg_primitive_summary(svg_path)
            self.assertTrue(view_box, f"{svg_path} <svg> must define viewBox")
            self.assertGreater(
                polygons + paths + rects,
                0,
                f"{svg_path} must contain geometric primitives (<polygon>/<path>/<rect>)",
            )
            self.assertTrue(
                any(color in lower for color in COBALT_PALETTE_HEX),
                f"{svg_path} must use canonical Cobalt brand palette hex colors",
            )

        # Canonical emblem parity between app favicon and site favicon
        app_summary = _svg_primitive_summary(app_svg)
        site_summary = _svg_primitive_summary(site_svg)
        self.assertEqual(
            app_summary,
            site_summary,
            f"Canonical emblem mismatch between {app_svg} {app_summary} and {site_svg} {site_summary}",
        )

        # Sidebar.svelte .sidebar-brand must render the Cobalt emblem
        # (inline <svg>, <Cobalt...> / <Brand...> component, or <img ... favicon> per cobalt_brand_and_ui_guide.md:146)
        sidebar_src = sidebar_svelte.read_text(encoding="utf-8")
        brand_match = re.search(
            r'<a[^>]*class="[^"]*sidebar-brand[^"]*"[^>]*>(.*?)</a>',
            sidebar_src,
            re.DOTALL,
        )
        self.assertIsNotNone(brand_match, "Sidebar.svelte missing .sidebar-brand anchor")
        brand_inner = brand_match.group(1)
        brand_lower = brand_inner.lower()
        has_inline_svg = "<svg" in brand_lower
        has_emblem_component = bool(re.search(r"<(?:cobalt|brand)[A-Za-z0-9_]*\b", brand_inner, re.IGNORECASE))
        favicon_import_match = re.search(
            r"import\s+([A-Za-z_][A-Za-z0-9_]*)\s+from\s+['\"][^'\"]*favicon[^'\"]*['\"]",
            sidebar_src,
        )
        has_favicon_img = bool(re.search(r"<img\b[^>]*favicon", brand_lower))
        if not has_favicon_img and favicon_import_match:
            imported_ident = favicon_import_match.group(1)
            has_favicon_img = bool(
                re.search(rf"<img\b[^>]*\b{re.escape(imported_ident)}\b", brand_inner)
            )
        self.assertTrue(
            has_inline_svg or has_emblem_component or has_favicon_img,
            "Sidebar.svelte .sidebar-brand must render a Cobalt emblem (inline <svg>, <Cobalt...> component, or <img ... favicon>)",
        )
        self.assertNotIn("#ff3e00", brand_lower)
        self.assertNotIn("svelte-logo", brand_lower)

    # ------------------------------------------------------------------
    # PKG-06: site/dist parity & zero markdown
    # ------------------------------------------------------------------
    def test_pkg_06_site_dist_parity_and_zero_markdown(self) -> None:
        dist_dir = REPO_ROOT / "site" / "dist"
        self.assertTrue(dist_dir.is_dir(), f"Missing {dist_dir}")

        required = ("index.html", "favicon.svg", "styles.css", "release-manifest.json", ".nojekyll")
        for fname in required:
            self.assertTrue((dist_dir / fname).is_file(), f"site/dist missing {fname}")

        md_files = [
            p.relative_to(dist_dir).as_posix()
            for p in dist_dir.rglob("*")
            if p.is_file() and p.suffix.lower() in (".md", ".markdown")
        ]
        self.assertEqual(md_files, [], f"site/dist contains disallowed markdown files: {md_files}")

        self.assertEqual(
            (dist_dir / "favicon.svg").read_bytes(),
            (REPO_ROOT / "site" / "favicon.svg").read_bytes(),
            "site/dist/favicon.svg is stale vs site/favicon.svg (re-run `node site/build.mjs`)",
        )
        self.assertEqual(
            (dist_dir / "styles.css").read_bytes(),
            (REPO_ROOT / "site" / "styles.css").read_bytes(),
            "site/dist/styles.css is stale vs site/styles.css",
        )
        self.assertEqual(
            json.loads((dist_dir / "release-manifest.json").read_text(encoding="utf-8")),
            json.loads((REPO_ROOT / "site" / "release-manifest.json").read_text(encoding="utf-8")),
            "site/dist/release-manifest.json is stale vs site/release-manifest.json",
        )

        index_html = (dist_dir / "index.html").read_text(encoding="utf-8")
        self.assertIn("favicon.svg", index_html)
        self.assertIn("styles.css", index_html)
        self.assertIn("Cobalt", index_html)
        self.assertNotIn("${", index_html, "Unescaped template placeholder found in site/dist/index.html")

        site_css = (REPO_ROOT / "site" / "styles.css").read_text(encoding="utf-8").lower()
        for token in ("#05070b", "#263c8a", "#1d63ed", "#faa031"):
            self.assertIn(token, site_css, f"site/styles.css missing canonical Cobalt token {token}")
        self.assertNotIn("#0a0e17", site_css, "site/styles.css still contains legacy #0a0e17 token")

        config_yml = (REPO_ROOT / "build" / "config.yml").read_text(encoding="utf-8")
        self.assertIn("Cobalt", config_yml, "build/config.yml productName must reflect Cobalt branding")
        self.assertIn('version: "1.0.0"', config_yml, "build/config.yml version must match release manifest 1.0.0")

    # ------------------------------------------------------------------
    # PKG-07: Brand guide & OKF / AGENTS.md indexing
    # ------------------------------------------------------------------
    def test_pkg_07_cobalt_brand_guide_and_indexing(self) -> None:
        guide_path = REPO_ROOT / "okf" / "02_present_state" / "cobalt_brand_and_ui_guide.md"
        self.assertTrue(guide_path.is_file(), f"Missing Brand Guide: {guide_path}")

        content = guide_path.read_text(encoding="utf-8")
        self.assertGreater(len(content.encode("utf-8")), 1000, "cobalt_brand_and_ui_guide.md is too short")

        lower = content.lower()
        self.assertIn("cobaltinvestissements.com", lower)
        self.assertIn("uni noir", lower)
        self.assertIn("ioskeleymono", lower)
        self.assertTrue(
            any(h in lower for h in ("#1d63ed", "#263c8a", "#38bdf8")),
            "Brand guide must document Cobalt blue palette hex codes",
        )
        self.assertTrue(
            any(term in lower for term in ("emblem", "emblème", "polygon", "geometric", "géométri")),
            "Brand guide must document Cobalt emblem geometry",
        )

        okf_index = (REPO_ROOT / "okf" / "00_index.md").read_text(encoding="utf-8")
        self.assertIn(
            "02_present_state/cobalt_brand_and_ui_guide.md",
            okf_index,
            "okf/00_index.md must link to 02_present_state/cobalt_brand_and_ui_guide.md",
        )

        agents_md = (REPO_ROOT / "AGENTS.md").read_text(encoding="utf-8")
        self.assertIn(
            "okf/02_present_state/cobalt_brand_and_ui_guide.md",
            agents_md,
            "AGENTS.md must index okf/02_present_state/cobalt_brand_and_ui_guide.md",
        )

    # ------------------------------------------------------------------
    # PKG-08: Repo hygiene, lockstep, i18n parity & V2 Wails bindings
    # ------------------------------------------------------------------
    def test_pkg_08_hygiene_lockstep_i18n_and_v2_bindings(self) -> None:
        hygiene = _load_script_module("check_okf_hygiene_v2", "check-okf-hygiene.py")
        lockstep = _load_script_module("check_dep_lockstep_v2", "check-dep-lockstep.py")
        i18n = _load_script_module("check_i18n_parity_v2", "check-i18n-parity.py")

        self.assertEqual(hygiene.check_hygiene(REPO_ROOT), [])
        self.assertEqual(lockstep.check_lockstep(REPO_ROOT), [])
        self.assertEqual(i18n.check_i18n_parity(REPO_ROOT), [])

        app_js = (REPO_ROOT / "frontend" / "bindings" / "tws_traderbot" / "app.js").read_text(encoding="utf-8")
        self.assertIn("export function ToggleStrategyPin", app_js, "V2 Wails binding ToggleStrategyPin missing")
        self.assertIn("export function MoveStrategyOrder", app_js, "V2 Wails binding MoveStrategyOrder missing")

        models_js = (
            REPO_ROOT / "frontend" / "bindings" / "tws_traderbot" / "backend" / "models" / "models.js"
        ).read_text(encoding="utf-8")
        for field in (
            "pinned",
            "sort_order",
            "account_id",
            "cash_usd",
            "cash_cad",
            "session_status",
            "open_positions",
            "open_orders",
            "broker_alerts",
        ):
            self.assertIn(
                f'"{field}"',
                models_js,
                f"V2 Wails model field {field!r} missing in frontend/bindings/.../models.js",
            )


if __name__ == "__main__":
    unittest.main()
