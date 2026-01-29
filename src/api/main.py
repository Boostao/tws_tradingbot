from __future__ import annotations

from collections import defaultdict, deque
import os
from threading import Lock
from time import time
import asyncio

from fastapi import Depends, FastAPI, HTTPException, Request
from fastapi.middleware.cors import CORSMiddleware
from fastapi.security import HTTPBasic, HTTPBasicCredentials
from starlette import status

from src.api.routers import backtest, config, notifications, state, strategy, symbols, watchlist, ws
from src.bot.tws_data_provider import get_tws_provider, reset_tws_provider
from src.config.settings import get_settings


_security = HTTPBasic()


def _auth_guard(credentials: HTTPBasicCredentials = Depends(_security)) -> None:
    settings = get_settings(force_reload=True)
    if not settings.auth.enabled:
        return
    if credentials.username != settings.auth.username or credentials.password != settings.auth.password:
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="invalid credentials")


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


def create_app() -> FastAPI:
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

    @app.middleware("http")
    async def rate_limit(request: Request, call_next):
        if rate_limit_enabled:
            path = request.url.path
            if path not in ("/health", "/docs", "/redoc", "/openapi.json"):
                client = request.client.host if request.client else "unknown"
                if not limiter.allow(client):
                    raise HTTPException(status_code=429, detail="rate limit exceeded")
        return await call_next(request)

    dependencies = [Depends(_auth_guard)] if settings.auth.enabled else None
    app.include_router(strategy.router, prefix="/api/v1", dependencies=dependencies)
    app.include_router(backtest.router, prefix="/api/v1", dependencies=dependencies)
    app.include_router(watchlist.router, prefix="/api/v1", dependencies=dependencies)
    app.include_router(state.router, prefix="/api/v1", dependencies=dependencies)
    app.include_router(config.router, prefix="/api/v1", dependencies=dependencies)
    app.include_router(notifications.router, prefix="/api/v1", dependencies=dependencies)
    app.include_router(symbols.router, prefix="/api/v1", dependencies=dependencies)
    app.include_router(ws.router)

    @app.get("/health")
    def health():
        return {"status": "ok"}

    @app.on_event("shutdown")
    async def shutdown_event() -> None:
        async def _shutdown_tws() -> None:
            try:
                provider = get_tws_provider()
                await asyncio.to_thread(provider.disconnect)
            except Exception:
                pass
            try:
                reset_tws_provider()
            except Exception:
                pass

        try:
            await asyncio.wait_for(_shutdown_tws(), timeout=3.0)
        except asyncio.TimeoutError:
            pass

    return app


app = create_app()
