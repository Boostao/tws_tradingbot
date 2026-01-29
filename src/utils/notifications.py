"""
Notification utilities for Telegram and Discord.
"""

from __future__ import annotations

import logging
import threading
import time
from typing import Callable, Optional

import requests

from src.config.settings import Settings
from src.config.settings import get_settings
from src.config.database import get_database


logger = logging.getLogger(__name__)


class NotificationManager:
    """Send notifications via Telegram and Discord based on configuration."""

    def __init__(self, settings: Settings):
        self._settings = settings
        self._telegram_listener: Optional[TelegramCommandListener] = None

    @property
    def enabled(self) -> bool:
        return bool(self._settings.notifications.enabled)

    def notify(self, message: str) -> None:
        if not self.enabled:
            return

        self._send_telegram(message)
        self._send_discord(message)
        self._record_event(message, level="info")

    def notify_status(self, status: str, mode: str) -> None:
        message = f"{status} | Mode: {mode}"
        self.notify(message)

    def notify_order(self, side: str, symbol: str, quantity: int, order_id: Optional[int]) -> None:
        order_part = f" (Order ID: {order_id})" if order_id is not None else ""
        self.notify(f"{side} {symbol} x{quantity}{order_part}")

    def notify_error(self, message: str) -> None:
        formatted = f"❌ Error: {message}"
        if not self.enabled:
            return
        self._send_telegram(formatted)
        self._send_discord(formatted)
        self._record_event(formatted, level="error")

    def start_command_listener(self, handler: Callable[[str], str]) -> None:
        cfg = self._settings.notifications.telegram
        if not (cfg.enabled and cfg.commands_enabled and cfg.bot_token and cfg.chat_id):
            return

        if self._telegram_listener and self._telegram_listener.is_running:
            return

        self._telegram_listener = TelegramCommandListener(
            bot_token=cfg.bot_token,
            chat_id=cfg.chat_id,
            poll_interval=cfg.poll_interval,
            handler=handler,
        )
        self._telegram_listener.start()

    def stop_command_listener(self) -> None:
        if self._telegram_listener:
            self._telegram_listener.stop()
            self._telegram_listener = None

    def _send_telegram(self, message: str) -> None:
        cfg = self._settings.notifications.telegram
        if not (cfg.enabled and cfg.bot_token and cfg.chat_id):
            return

        url = f"https://api.telegram.org/bot{cfg.bot_token}/sendMessage"
        payload = {"chat_id": cfg.chat_id, "text": message}

        try:
            response = requests.post(url, json=payload, timeout=10)
            if response.status_code >= 400:
                logger.warning("Telegram notification failed: %s", response.text)
        except Exception as exc:
            logger.warning("Telegram notification error: %s", exc)

    def _send_discord(self, message: str) -> None:
        cfg = self._settings.notifications.discord
        if not (cfg.enabled and cfg.webhook_url):
            return

        try:
            response = requests.post(
                cfg.webhook_url,
                json={"content": message},
                timeout=10,
            )
            if response.status_code >= 400:
                logger.warning("Discord notification failed: %s", response.text)
        except Exception as exc:
            logger.warning("Discord notification error: %s", exc)

    def _record_event(self, message: str, level: str = "info") -> None:
        try:
            settings = get_settings(force_reload=True)
            if not settings.database.enabled:
                return
            db = get_database()
            db.add_notification(level=level, message=message, channel=None)
        except Exception as exc:
            logger.debug("Notification persistence failed: %s", exc)


class TelegramCommandListener:
    """Poll Telegram for bot commands and invoke a handler."""

    def __init__(
        self,
        bot_token: str,
        chat_id: str,
        poll_interval: int,
        handler: Callable[[str], str],
    ) -> None:
        self._bot_token = bot_token
        self._chat_id = str(chat_id)
        self._poll_interval = max(1, int(poll_interval))
        self._handler = handler
        self._stop_event = threading.Event()
        self._thread: Optional[threading.Thread] = None
        self._offset: Optional[int] = None

    @property
    def is_running(self) -> bool:
        return self._thread is not None and self._thread.is_alive()

    def start(self) -> None:
        if self.is_running:
            return
        self._stop_event.clear()
        self._thread = threading.Thread(target=self._run, daemon=True)
        self._thread.start()

    def stop(self) -> None:
        self._stop_event.set()
        if self._thread:
            self._thread.join(timeout=5)

    def _run(self) -> None:
        while not self._stop_event.is_set():
            try:
                updates = self._get_updates()
                for update in updates:
                    self._offset = update.get("update_id", 0) + 1
                    message = update.get("message", {})
                    text = (message.get("text") or "").strip()
                    chat_id = str(message.get("chat", {}).get("id", ""))

                    if not text.startswith("/"):
                        continue
                    if chat_id != self._chat_id:
                        continue

                    response = self._handler(text)
                    if response:
                        self._send_message(response)
            except Exception as exc:
                logger.warning("Telegram command polling error: %s", exc)

            time.sleep(self._poll_interval)

    def _get_updates(self) -> list[dict]:
        url = f"https://api.telegram.org/bot{self._bot_token}/getUpdates"
        params = {"timeout": 5}
        if self._offset is not None:
            params["offset"] = self._offset
        response = requests.get(url, params=params, timeout=10)
        response.raise_for_status()
        data = response.json()
        return data.get("result", [])

    def _send_message(self, message: str) -> None:
        url = f"https://api.telegram.org/bot{self._bot_token}/sendMessage"
        payload = {"chat_id": self._chat_id, "text": message}
        requests.post(url, json=payload, timeout=10)