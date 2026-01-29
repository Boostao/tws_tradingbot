from __future__ import annotations

from fastapi.testclient import TestClient

from src.api.main import create_app


client = TestClient(create_app())


def test_ws_state_stream() -> None:
    with client.websocket_connect("/ws/state") as ws:
        payload = ws.receive_json()
        assert isinstance(payload, dict)
        assert "status" in payload


def test_ws_logs_stream() -> None:
    with client.websocket_connect("/ws/logs") as ws:
        payload = ws.receive_json()
        assert isinstance(payload, dict)
        assert "logs" in payload


def test_ws_notifications_stream() -> None:
    with client.websocket_connect("/ws/notifications") as ws:
        payload = ws.receive_json()
        assert isinstance(payload, dict)
        assert "events" in payload
