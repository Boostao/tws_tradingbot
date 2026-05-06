from src.api import utils


def test_normalize_cockpit_state_keeps_one_active_strategy_per_workspace(monkeypatch) -> None:
    monkeypatch.setattr(utils, "load_watchlist_state", lambda: {"feed": None})
    monkeypatch.setattr(
        utils,
        "list_strategy_library",
        lambda: [
            {"id": "strategy-a", "name": "Strategy A", "rule_count": 1, "enabled_rule_count": 1, "source": "library"},
            {"id": "strategy-b", "name": "Strategy B", "rule_count": 1, "enabled_rule_count": 1, "source": "library"},
        ],
    )

    normalized = utils._normalize_cockpit_state(
        {
            "global_enabled": True,
            "active_workspace_id": "workspace-long",
            "workspaces": [
                {
                    "id": "workspace-long",
                    "name": "Long",
                    "kind": "long",
                    "enabled": True,
                    "strategy_slots": [
                        {"id": "slot-1", "label": "One", "strategy_id": "strategy-a", "enabled": True},
                        {"id": "slot-2", "label": "Two", "strategy_id": "strategy-b", "enabled": True},
                    ],
                }
            ],
        }
    )

    workspace = normalized["workspaces"][0]
    assert len(workspace["strategy_slots"]) == 1
    assert workspace["strategy_slots"][0]["id"] == "slot-1"
    assert workspace["strategy_slots"][0]["label"] == "Active Strategy"
    assert workspace["strategy_slots"][0]["strategy_id"] == "strategy-a"
    assert workspace["strategy_slots"][0]["enabled"] is True


def test_normalize_cockpit_state_disables_slot_without_strategy(monkeypatch) -> None:
    monkeypatch.setattr(utils, "load_watchlist_state", lambda: {"feed": None})
    monkeypatch.setattr(
        utils,
        "list_strategy_library",
        lambda: [{"id": "strategy-a", "name": "Strategy A", "rule_count": 1, "enabled_rule_count": 1, "source": "library"}],
    )

    normalized = utils._normalize_cockpit_state(
        {
            "workspaces": [
                {
                    "id": "workspace-long",
                    "name": "Long",
                    "kind": "long",
                    "enabled": True,
                    "strategy_slots": [{"id": "slot-9", "label": "Ignored", "strategy_id": None, "enabled": True}],
                }
            ]
        }
    )

    workspace = normalized["workspaces"][0]
    assert len(workspace["strategy_slots"]) == 1
    assert workspace["strategy_slots"][0]["strategy_id"] is None
    assert workspace["strategy_slots"][0]["enabled"] is False


def test_normalize_cockpit_state_keeps_only_active_workspace_strategy_enabled(monkeypatch) -> None:
    monkeypatch.setattr(utils, "load_watchlist_state", lambda: {"feed": None})
    monkeypatch.setattr(
        utils,
        "list_strategy_library",
        lambda: [
            {"id": "strategy-a", "name": "Strategy A", "rule_count": 1, "enabled_rule_count": 1, "source": "library"},
            {"id": "strategy-b", "name": "Strategy B", "rule_count": 1, "enabled_rule_count": 1, "source": "library"},
        ],
    )

    normalized = utils._normalize_cockpit_state(
        {
            "global_enabled": True,
            "active_workspace_id": "workspace-short",
            "workspaces": [
                {
                    "id": "workspace-long",
                    "name": "Long",
                    "kind": "long",
                    "enabled": True,
                    "strategy_slots": [{"id": "slot-1", "label": "One", "strategy_id": "strategy-a", "enabled": True}],
                },
                {
                    "id": "workspace-short",
                    "name": "Short",
                    "kind": "short",
                    "enabled": True,
                    "strategy_slots": [{"id": "slot-1", "label": "One", "strategy_id": "strategy-b", "enabled": True}],
                },
            ],
        }
    )

    assert normalized["active_workspace_id"] == "workspace-short"
    long_workspace = next(workspace for workspace in normalized["workspaces"] if workspace["id"] == "workspace-long")
    short_workspace = next(workspace for workspace in normalized["workspaces"] if workspace["id"] == "workspace-short")
    assert long_workspace["strategy_slots"][0]["enabled"] is False
    assert short_workspace["strategy_slots"][0]["enabled"] is True