import logging
import logging.handlers
import sys
from pathlib import Path
from typing import Optional, List, Callable

from src.config.settings import get_settings


# Module-level log buffer for UI display
_log_buffer: List[str] = []
_log_buffer_max_size: int = 100
_log_callbacks: List[Callable[[str], None]] = []


class Logger:
    """Centralized logging configuration for the trading bot."""

    _instance: Optional['Logger'] = None
    _logger: Optional[logging.Logger] = None

    def __new__(cls) -> 'Logger':
        if cls._instance is None:
            cls._instance = super().__new__(cls)
        return cls._instance

    def __init__(self):
        if self._logger is None:
            self._setup_logger()

    def _setup_logger(self) -> None:
        """Set up the logger with configuration from settings."""
        settings = get_settings()
        log_config = settings.logging
        log_level = getattr(logging, log_config.level.upper(), logging.INFO)
        log_format = log_config.format

        root_logger = logging.getLogger()
        root_logger.setLevel(log_level)
        root_logger.handlers = [
            handler for handler in root_logger.handlers if not getattr(handler, "_traderbot_owned", False)
        ]
        has_external_console_handler = any(
            isinstance(handler, logging.StreamHandler) and not getattr(handler, "_traderbot_owned", False)
            for handler in root_logger.handlers
        )

        formatter = logging.Formatter(log_format)

        if log_config.file_enabled:
            file_path = Path(log_config.file_path)
            file_path.parent.mkdir(parents=True, exist_ok=True)

            daily_handler = logging.handlers.TimedRotatingFileHandler(
                file_path,
                when='midnight',
                interval=1,
                backupCount=log_config.file_backup_count,
                encoding='utf-8',
            )
            daily_handler.suffix = "%Y-%m-%d"
            daily_handler.setLevel(log_level)
            daily_handler.setFormatter(formatter)
            daily_handler._traderbot_owned = True
            root_logger.addHandler(daily_handler)

        if log_config.console_enabled and not has_external_console_handler:
            console_level = getattr(logging, log_config.console_level.upper(), logging.INFO)

            console_handler = logging.StreamHandler(sys.stdout)
            console_handler.setLevel(console_level)
            console_handler.setFormatter(formatter)
            console_handler._traderbot_owned = True
            root_logger.addHandler(console_handler)

        buffer_handler = LogBufferHandler()
        buffer_handler.setLevel(log_level)
        buffer_handler.setFormatter(logging.Formatter(
            "[%(asctime)s] [%(levelname)s] %(message)s",
            datefmt="%Y-%m-%d %H:%M:%S"
        ))
        buffer_handler._traderbot_owned = True
        root_logger.addHandler(buffer_handler)

        self._logger = root_logger

    def get_logger(self, name: str) -> logging.Logger:
        """Get a logger instance for a specific module."""
        if self._logger is None:
            self._setup_logger()

        return logging.getLogger(name)

    @property
    def logger(self) -> logging.Logger:
        """Get the main logger instance."""
        if self._logger is None:
            self._setup_logger()
        return self._logger


class LogBufferHandler(logging.Handler):
    """
    Custom logging handler that stores logs in a buffer for UI display.
    
    This handler appends formatted log messages to a module-level buffer
    that can be retrieved by the UI for display.
    """
    
    def emit(self, record: logging.LogRecord) -> None:
        """Emit a log record to the buffer."""
        global _log_buffer
        try:
            msg = self.format(record)
            _log_buffer.append(msg)
            
            # Trim buffer to max size
            if len(_log_buffer) > _log_buffer_max_size:
                _log_buffer = _log_buffer[-_log_buffer_max_size:]
            
            # Call any registered callbacks
            for callback in _log_callbacks:
                try:
                    callback(msg)
                except Exception:
                    pass
                    
        except Exception:
            self.handleError(record)


def get_log_buffer() -> List[str]:
    """
    Get the current log buffer contents.
    
    Returns:
        List of recent log messages
    """
    return _log_buffer.copy()


def clear_log_buffer() -> None:
    """Clear the log buffer."""
    global _log_buffer
    _log_buffer = []


def register_log_callback(callback: Callable[[str], None]) -> None:
    """
    Register a callback to be called for each new log message.
    
    Args:
        callback: Function that takes a log message string
    """
    if callback not in _log_callbacks:
        _log_callbacks.append(callback)


def unregister_log_callback(callback: Callable[[str], None]) -> None:
    """
    Unregister a log callback.
    
    Args:
        callback: Previously registered callback function
    """
    if callback in _log_callbacks:
        _log_callbacks.remove(callback)


# Global logger instance (lazy init to avoid import-time side effects)
logger_instance: Optional[Logger] = None


def _get_logger_instance() -> Logger:
    global logger_instance
    if logger_instance is None:
        logger_instance = Logger()
    return logger_instance

def get_logger(name: str) -> logging.Logger:
    """Convenience function to get a logger for a module."""
    return _get_logger_instance().get_logger(name)


def setup_logging(
    level: int | None = None,
    format_str: Optional[str] = None,
) -> None:
    """
    Set up basic logging configuration.
    
    This is a simpler alternative to the Logger class for quick setup.
    
    Args:
        level: Logging level (e.g., logging.DEBUG, logging.INFO)
        format_str: Custom format string (uses default if None)
    """
    if level is None and format_str is None:
        _get_logger_instance()
    else:
        if format_str is None:
            format_str = "%(asctime)s - %(name)s - %(levelname)s - %(message)s"

        root_logger = logging.getLogger()
        root_logger.handlers = [
            handler for handler in root_logger.handlers if not getattr(handler, "_traderbot_owned", False)
        ]
        root_logger.setLevel(level or logging.INFO)
        console_handler = logging.StreamHandler(sys.stdout)
        console_handler.setFormatter(logging.Formatter(format_str))
        console_handler._traderbot_owned = True
        root_logger.addHandler(console_handler)

    logging.getLogger("urllib3").setLevel(logging.WARNING)
    logging.getLogger("asyncio").setLevel(logging.WARNING)
