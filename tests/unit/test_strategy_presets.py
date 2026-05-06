from __future__ import annotations

from src.api import utils as api_utils
from src.bot.strategy.rules.models import Strategy


def _strategy(name: str, strategy_id: str) -> Strategy:
    return Strategy(id=strategy_id, name=name, tickers=[], rules=[])


def test_save_strategy_preset_creates_a_new_snapshot(monkeypatch, tmp_path) -> None:
    monkeypatch.setattr(api_utils, "_strategies_dir_path", lambda: tmp_path)

    original = _strategy("Momentum Builder", "active-strategy")
    entry = api_utils.save_strategy_preset(original, "Momentum Preset")

    saved = api_utils.load_strategy_preset(entry["id"])

    assert entry["id"] != original.id
    assert saved.id == entry["id"]
    assert saved.name == "Momentum Preset"
    assert list(tmp_path.glob("*.json")) == [tmp_path / f"{entry['id']}.json"]


def test_update_strategy_preset_overwrites_existing_snapshot(monkeypatch, tmp_path) -> None:
    monkeypatch.setattr(api_utils, "_strategies_dir_path", lambda: tmp_path)

    created = api_utils.save_strategy_preset(_strategy("Momentum Preset", "active-strategy"), "Momentum Preset")
    original_snapshot = api_utils.load_strategy_preset(created["id"])

    updated = api_utils.update_strategy_preset(
        created["id"],
        _strategy("Momentum Preset v2", "different-working-copy"),
        "Momentum Preset v2",
    )
    saved = api_utils.load_strategy_preset(created["id"])

    assert updated["id"] == created["id"]
    assert saved.id == created["id"]
    assert saved.name == "Momentum Preset v2"
    assert saved.created_at == original_snapshot.created_at
    assert list(tmp_path.glob("*.json")) == [tmp_path / f"{created['id']}.json"]