from __future__ import annotations

from collections import defaultdict, deque
import logging
import os
from pathlib import Path
from threading import Lock
from time import time

from fastapi import FastAPI, HTTPException, Request
from fastapi.middleware.cors import CORSMiddleware

from src.api.routers import cockpit, config, diagnostics, state, strategy, symbols, watchlist
from src.config.settings import get_settings
from src.utils.logger import setup_logging


logger = logging.getLogger(__name__)


class _RateLimiter:
    def __init__(self, rps: int, burst: int) -> None:
        self.rps = max(1, rps)
        self.burst = max(1, burst)
        self.window = 1.0
        self._buckets: dict[str, deque[float]] = defaultdict(deque)
        self._lock = Lock()

    def allow(self, key: str) -> bool:
        now = time()
        with self._lock:
            bucket = self._buckets[key]
            while bucket and now - bucket[0] > self.window:
                bucket.popleft()
            if len(bucket) >= self.burst:
                return False
            bucket.append(now)
            return True


def _mask_account(account: str | None) -> str:
    raw = str(account or "").strip()
    if not raw:
        return "unset"
    if len(raw) <= 4:
        return raw
    return f"{raw[:2]}***{raw[-2:]}"


def _log_api_startup(settings, rate_limit_enabled: bool, cors_origins: list[str], cors_origin_regex: str) -> None:
    logger.info(
        "API startup | env=%s ib_host=%s ib_port=%s trading_mode=%s account=%s rate_limit=%s cors_origins=%s cors_regex=%s log_file=%s",
        os.getenv("TRADING_BOT_ENV", "development"),
        settings.ib.host,
        settings.ib.port,
        settings.ib.trading_mode,
        _mask_account(settings.ib.account),
        rate_limit_enabled,
        len(cors_origins),
        cors_origin_regex,
        Path(settings.logging.file_path),
    )


def create_app() -> FastAPI:
    setup_logging()
    app = FastAPI(title="TWS Traderbot API", version="0.1.0")

    settings = get_settings(force_reload=True)
    cors_origins = [
        origin.strip()
        for origin in os.getenv("API_CORS_ORIGINS", "http://localhost:5173,http://127.0.0.1:5173").split(",")
        if origin.strip()
    ]
    cors_origin_regex = os.getenv(
        "API_CORS_ORIGIN_REGEX",
        r"^http://(localhost|127\.0\.0\.1|\d{1,3}(?:\.\d{1,3}){3}):5173$",
    )

    app.add_middleware(
        CORSMiddleware,
        allow_origins=cors_origins,
        allow_origin_regex=cors_origin_regex,
        allow_credentials=True,
        allow_methods=["*"],
        allow_headers=["*"],
    )

    rate_limit_enabled = os.getenv("API_RATE_LIMIT_ENABLED", "false").lower() == "true"
    limiter = _RateLimiter(
        rps=int(os.getenv("API_RATE_LIMIT_RPS", "5")),
        burst=int(os.getenv("API_RATE_LIMIT_BURST", "20")),
    )

    @app.on_event("startup")
    async def log_startup() -> None:
        _log_api_startup(settings, rate_limit_enabled, cors_origins, cors_origin_regex)

    @app.middleware("http")
    async def rate_limit(request: Request, call_next):
        if rate_limit_enabled:
            path = request.url.path
            if path not in ("/health", "/docs", "/redoc", "/openapi.json"):
                client = request.client.host if request.client else "unknown"
                if not limiter.allow(client):
                    raise HTTPException(status_code=429, detail="rate limit exceeded")
        return await call_next(request)

    app.include_router(strategy.router, prefix="/api/v1")
    app.include_router(cockpit.router, prefix="/api/v1")
    app.include_router(watchlist.router, prefix="/api/v1")
    app.include_router(symbols.router, prefix="/api/v1")
    app.include_router(diagnostics.router, prefix="/api/v1")
    app.include_router(config.router, prefix="/api/v1")
    app.include_router(state.router, prefix="/api/v1")

    @app.get("/health")
    def health():
        return {"status": "ok"}

    return app


app = create_app()
