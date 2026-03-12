"""Pytest session hooks for the PineScript-oriented codebase."""

from __future__ import annotations

def pytest_sessionfinish(session, exitstatus):
    """No-op: no runtime bot threads/processes are used in this branch."""
    return