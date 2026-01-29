"""Pytest session cleanup to avoid hanging background threads."""

from __future__ import annotations

import logging


logger = logging.getLogger(__name__)


def pytest_sessionfinish(session, exitstatus):
    """Ensure background resources are stopped before pytest exits."""
    try:
        from src.bot.data_provider import data_provider
        data_provider.disconnect()
    except Exception as exc:  # pragma: no cover - cleanup best-effort
        logger.debug("data_provider disconnect failed: %s", exc)

    try:
        from src.bot.tws_data_provider import reset_tws_provider
        reset_tws_provider()
    except Exception as exc:  # pragma: no cover - cleanup best-effort
        logger.debug("tws_data_provider reset failed: %s", exc)

    try:
        from src.bot.engine import engine
        engine.stop()
    except Exception as exc:  # pragma: no cover - cleanup best-effort
        logger.debug("engine stop failed: %s", exc)