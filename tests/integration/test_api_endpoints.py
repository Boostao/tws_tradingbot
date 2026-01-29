from __future__ import annotations

from fastapi.testclient import TestClient

from src.api.main import create_app


client = TestClient(create_app())


def test_health_check() -> None:
    response = client.get("/health")
    assert response.status_code == 200
    payload = response.json()
    assert payload.get("status") == "ok"


def test_state_and_logs() -> None:
    state_response = client.get("/api/v1/state")
    assert state_response.status_code == 200
    state = state_response.json()
    assert isinstance(state, dict)
    assert "status" in state

    logs_response = client.get("/api/v1/logs")
    assert logs_response.status_code == 200
    logs = logs_response.json()
    assert isinstance(logs.get("logs"), list)


def test_strategy_get_and_validate() -> None:
    response = client.get("/api/v1/strategy")
    assert response.status_code == 200
    strategy = response.json()
    assert isinstance(strategy, dict)
    assert "name" in strategy

    validate = client.post("/api/v1/strategy/validate", json=strategy)
    assert validate.status_code == 200
    payload = validate.json()
    assert "valid" in payload
    assert "errors" in payload


def test_config_get() -> None:
    response = client.get("/api/v1/config")
    assert response.status_code == 200
    payload = response.json()
    assert isinstance(payload, dict)


def test_watchlist_get() -> None:
    response = client.get("/api/v1/watchlist")
    assert response.status_code == 200
    payload = response.json()
    assert isinstance(payload.get("symbols"), list)


def test_notifications_list() -> None:
    response = client.get("/api/v1/notifications", params={"page": 1, "page_size": 5})
    assert response.status_code == 200
    payload = response.json()
    assert "events" in payload
    assert "total" in payload
    assert payload.get("page") == 1
