"""
Configuration management for the Trading Bot.

Loads settings from YAML files, environment variables, and DuckDB database,
providing typed dataclasses for easy access.
"""

import os
import yaml
from dataclasses import dataclass, field, asdict
from pathlib import Path
from typing import Dict, Any, Optional
from dotenv import load_dotenv


@dataclass
class IBConfig:
    """Interactive Brokers connection configuration."""
    host: str = "127.0.0.1"
    port: int = 4002  # 4002 for IB Gateway paper, 7497 for TWS paper
    client_id: int = 1
    account: str = ""
    timeout: int = 5
    trading_mode: str = "paper"  # paper | live

    @property
    def is_paper_trading(self) -> bool:
        return self.trading_mode == "paper"


@dataclass
class AppConfig:
    """Application-level configuration."""
    log_level: str = "INFO"
    strategies_dir: str = "strategies"
    active_strategy_path: str = "config/active_strategy.json"
    watchlist_path: str = "config/watchlist.txt"
    symbol_cache_path: str = "data/symbol_cache.json"
    data_dir: str = "data"
    logs_dir: str = "logs"


@dataclass
class BacktestConfig:
    """Backtesting configuration."""
    default_capital: float = 10000.0
    data_dir: str = "data/historical"
    default_commission: float = 0.001
    slippage_model: str = "fixed"


@dataclass
class RiskConfig:
    """Risk management configuration."""
    max_drawdown: float = 0.10
    daily_loss_limit: float = 0.05
    max_position_pct: float = 0.20
    use_stop_loss: bool = True
    stop_loss_pct: float = 0.02


@dataclass
class UIConfig:
    """UI configuration."""
    port: int = 8501
    host: str = "0.0.0.0"
    refresh_interval: int = 5
    theme: str = "dark"


@dataclass
class AuthConfig:
    """Authentication configuration for the UI."""
    enabled: bool = False
    username: str = "admin"
    password: str = "change-me"


@dataclass
class DatabaseConfig:
    """DuckDB database configuration."""
    enabled: bool = True
    path: str = "data/traderbot.duckdb"
    sync_on_start: bool = True  # Sync YAML config to DB on startup
    use_db_for_state: bool = True  # Use DB for bot state instead of JSON files


@dataclass
class LoggingConfig:
    """Logging configuration."""
    level: str = "INFO"
    format: str = "%(asctime)s - %(name)s - %(levelname)s - %(message)s"
    file_enabled: bool = True
    file_path: str = "logs/trading_bot.log"
    file_max_bytes: int = 10485760
    file_backup_count: int = 5
    console_enabled: bool = True
    console_level: str = "INFO"


@dataclass
class TelegramConfig:
    """Telegram notification configuration."""
    enabled: bool = False
    bot_token: str = ""
    chat_id: str = ""
    commands_enabled: bool = False
    poll_interval: int = 5


@dataclass
class DiscordConfig:
    """Discord notification configuration."""
    enabled: bool = False
    webhook_url: str = ""


@dataclass
class NotificationsConfig:
    """Notification configuration."""
    enabled: bool = False
    telegram: TelegramConfig = field(default_factory=TelegramConfig)
    discord: DiscordConfig = field(default_factory=DiscordConfig)


@dataclass
class Settings:
    """Main settings container with all configuration sections."""
    ib: IBConfig = field(default_factory=IBConfig)
    app: AppConfig = field(default_factory=AppConfig)
    backtest: BacktestConfig = field(default_factory=BacktestConfig)
    risk: RiskConfig = field(default_factory=RiskConfig)
    ui: UIConfig = field(default_factory=UIConfig)
    auth: AuthConfig = field(default_factory=AuthConfig)
    database: DatabaseConfig = field(default_factory=DatabaseConfig)
    logging: LoggingConfig = field(default_factory=LoggingConfig)
    notifications: NotificationsConfig = field(default_factory=NotificationsConfig)

    # Store raw config for backward compatibility
    _raw_config: Dict[str, Any] = field(default_factory=dict, repr=False)

    def get(self, key: str, default: Any = None) -> Any:
        """Get a configuration value by dot-separated key."""
        keys = key.split('.')
        current = self._raw_config
        for k in keys:
            if isinstance(current, dict) and k in current:
                current = current[k]
            else:
                return default
        return current


class ConfigLoader:
    """Loads and manages configuration from multiple sources."""

    def __init__(self, config_dir: Optional[Path] = None):
        env_config_dir = os.getenv("TRADERBOT_CONFIG_DIR")
        default_dir = Path(__file__).parent.parent.parent / "config"
        self.config_dir = Path(env_config_dir) if env_config_dir else (config_dir or default_dir)
        self.project_root = Path(__file__).parent.parent.parent

        # Load environment variables
        dotenv_path = os.getenv("TRADERBOT_DOTENV_PATH")
        if dotenv_path:
            load_dotenv(dotenv_path=dotenv_path)
        load_dotenv(dotenv_path=self.project_root / ".env")
        load_dotenv()

    def load(self) -> Settings:
        """Load configuration and return a Settings object."""
        raw_config = self._load_raw_config()
        return self._create_settings(raw_config)

    def _load_raw_config(self) -> Dict[str, Any]:
        """Load raw configuration from YAML and environment."""
        # Load default config
        config = self._load_yaml_file("default.yaml")

        # Load environment-specific overrides
        env = os.getenv("TRADING_BOT_ENV", "development")
        env_config = self._load_yaml_file(f"environment/{env}.yaml")
        config = self._deep_merge(config, env_config)

        # Apply environment variable overrides
        self._apply_env_overrides(config)

        return config

    def _load_yaml_file(self, filename: str) -> Dict[str, Any]:
        """Load a YAML file from the config directory."""
        file_path = self.config_dir / filename
        if file_path.exists():
            with open(file_path, 'r') as f:
                return yaml.safe_load(f) or {}
        return {}

    def _deep_merge(self, base: Dict[str, Any], override: Dict[str, Any]) -> Dict[str, Any]:
        """Deep merge two dictionaries."""
        result = base.copy()
        for key, value in override.items():
            if key in result and isinstance(result[key], dict) and isinstance(value, dict):
                result[key] = self._deep_merge(result[key], value)
            else:
                result[key] = value
        return result

    def _apply_env_overrides(self, config: Dict[str, Any]) -> None:
        """Apply environment variable overrides."""
        env_mappings = {
            "IB_HOST": ("ib", "host"),
            "IB_PORT": ("ib", "port"),
            "IB_CLIENT_ID": ("ib", "client_id"),
            "IB_ACCOUNT": ("ib", "account"),
            "IB_TRADING_MODE": ("ib", "trading_mode"),
            "LOG_LEVEL": ("app", "log_level"),
            "APP_ACTIVE_STRATEGY_PATH": ("app", "active_strategy_path"),
            "APP_WATCHLIST_PATH": ("app", "watchlist_path"),
            "APP_SYMBOL_CACHE_PATH": ("app", "symbol_cache_path"),
            "BACKTEST_CAPITAL": ("backtest", "default_capital"),
            "AUTH_ENABLED": ("auth", "enabled"),
            "AUTH_USERNAME": ("auth", "username"),
            "AUTH_PASSWORD": ("auth", "password"),
            "NOTIFICATIONS_ENABLED": ("notifications", "enabled"),
            "TELEGRAM_ENABLED": ("notifications", "telegram", "enabled"),
            "TELEGRAM_BOT_TOKEN": ("notifications", "telegram", "bot_token"),
            "TELEGRAM_CHAT_ID": ("notifications", "telegram", "chat_id"),
            "TELEGRAM_COMMANDS_ENABLED": ("notifications", "telegram", "commands_enabled"),
            "TELEGRAM_POLL_INTERVAL": ("notifications", "telegram", "poll_interval"),
            "DISCORD_ENABLED": ("notifications", "discord", "enabled"),
            "DISCORD_WEBHOOK_URL": ("notifications", "discord", "webhook_url"),
        }

        for env_key, config_path in env_mappings.items():
            value = os.getenv(env_key)
            if value is not None:
                self._set_nested(config, config_path, self._convert_value(value))

    def _set_nested(self, config: Dict[str, Any], path: tuple, value: Any) -> None:
        """Set a nested value in config dict."""
        current = config
        for key in path[:-1]:
            if key not in current:
                current[key] = {}
            current = current[key]
        current[path[-1]] = value

    def _convert_value(self, value: str) -> Any:
        """Convert string value to appropriate type."""
        if value.lower() in ('true', 'false'):
            return value.lower() == 'true'
        try:
            if '.' not in value:
                return int(value)
            return float(value)
        except ValueError:
            return value

    def _create_settings(self, raw_config: Dict[str, Any]) -> Settings:
        """Create Settings object from raw config dict."""
        ib_cfg = raw_config.get("ib", {})
        app_cfg = raw_config.get("app", {})
        backtest_cfg = raw_config.get("backtest", {})
        risk_cfg = raw_config.get("risk", {})
        ui_cfg = raw_config.get("ui", {})
        auth_cfg = raw_config.get("auth", {})
        database_cfg = raw_config.get("database", {})
        logging_cfg = raw_config.get("logging", {})
        notifications_cfg = raw_config.get("notifications", {})

        return Settings(
            ib=IBConfig(
                host=ib_cfg.get("host", "127.0.0.1"),
                port=ib_cfg.get("port", 4002),
                client_id=ib_cfg.get("client_id", 1),
                account=ib_cfg.get("account", ""),
                timeout=ib_cfg.get("timeout", 30),
                trading_mode=ib_cfg.get("trading_mode", "paper"),
            ),
            app=AppConfig(
                log_level=app_cfg.get("log_level", "INFO"),
                strategies_dir=app_cfg.get("strategies_dir", "strategies"),
                active_strategy_path=app_cfg.get("active_strategy_path", "config/active_strategy.json"),
                watchlist_path=app_cfg.get("watchlist_path", "config/watchlist.txt"),
                symbol_cache_path=app_cfg.get("symbol_cache_path", "data/symbol_cache.json"),
                data_dir=app_cfg.get("data_dir", "data"),
                logs_dir=app_cfg.get("logs_dir", "logs"),
            ),
            backtest=BacktestConfig(
                default_capital=backtest_cfg.get("default_capital", 10000.0),
                data_dir=backtest_cfg.get("data_dir", "data/historical"),
                default_commission=backtest_cfg.get("default_commission", 0.001),
                slippage_model=backtest_cfg.get("slippage_model", "fixed"),
            ),
            risk=RiskConfig(
                max_drawdown=risk_cfg.get("max_drawdown", 0.10),
                daily_loss_limit=risk_cfg.get("daily_loss_limit", 0.05),
                max_position_pct=risk_cfg.get("max_position_pct", 0.20),
                use_stop_loss=risk_cfg.get("use_stop_loss", True),
                stop_loss_pct=risk_cfg.get("stop_loss_pct", 0.02),
            ),
            ui=UIConfig(
                port=ui_cfg.get("port", 8501),
                host=ui_cfg.get("host", "0.0.0.0"),
                refresh_interval=ui_cfg.get("refresh_interval", 5),
                theme=ui_cfg.get("theme", "dark"),
            ),
            auth=AuthConfig(
                enabled=auth_cfg.get("enabled", False),
                username=auth_cfg.get("username", "admin"),
                password=auth_cfg.get("password", "change-me"),
            ),
            database=DatabaseConfig(
                enabled=database_cfg.get("enabled", True),
                path=database_cfg.get("path", "data/traderbot.duckdb"),
                sync_on_start=database_cfg.get("sync_on_start", True),
                use_db_for_state=database_cfg.get("use_db_for_state", True),
            ),
            logging=LoggingConfig(
                level=logging_cfg.get("level", "INFO"),
                format=logging_cfg.get("format", "%(asctime)s - %(name)s - %(levelname)s - %(message)s"),
                file_enabled=logging_cfg.get("file", {}).get("enabled", True),
                file_path=logging_cfg.get("file", {}).get("path", "logs/trading_bot.log"),
                file_max_bytes=logging_cfg.get("file", {}).get("max_bytes", 10485760),
                file_backup_count=logging_cfg.get("file", {}).get("backup_count", 5),
                console_enabled=logging_cfg.get("console", {}).get("enabled", True),
                console_level=logging_cfg.get("console", {}).get("level", "INFO"),
            ),
            notifications=NotificationsConfig(
                enabled=notifications_cfg.get("enabled", False),
                telegram=TelegramConfig(
                    enabled=notifications_cfg.get("telegram", {}).get("enabled", False),
                    bot_token=notifications_cfg.get("telegram", {}).get("bot_token", ""),
                    chat_id=notifications_cfg.get("telegram", {}).get("chat_id", ""),
                    commands_enabled=notifications_cfg.get("telegram", {}).get("commands_enabled", False),
                    poll_interval=notifications_cfg.get("telegram", {}).get("poll_interval", 5),
                ),
                discord=DiscordConfig(
                    enabled=notifications_cfg.get("discord", {}).get("enabled", False),
                    webhook_url=notifications_cfg.get("discord", {}).get("webhook_url", ""),
                ),
            ),
            _raw_config=raw_config,
        )


def load_config(config_dir: Optional[Path] = None, sync_to_db: bool = True) -> Settings:
    """Load configuration and return Settings object.
    
    This is the main entry point for loading configuration.
    Configuration is loaded from YAML files first, then optionally synced to DuckDB.
    
    Args:
        config_dir: Optional path to config directory. Defaults to project's config/ folder.
        sync_to_db: Whether to sync configuration to DuckDB (default: True).
        
    Returns:
        Settings object with all configuration values.
    """
    loader = ConfigLoader(config_dir)
    settings = loader.load()
    
    # Sync to database if enabled
    if sync_to_db and settings.database.enabled and settings.database.sync_on_start:
        try:
            from src.config.database import get_database
            db_path = Path(settings.database.path)
            if not db_path.is_absolute():
                db_path = loader.project_root / db_path
            
            db = get_database(
                db_path,
            )
            
            # Sync each configuration section to database
            db.set_section_config("ib", asdict(settings.ib))
            db.set_section_config("app", asdict(settings.app))
            db.set_section_config("backtest", asdict(settings.backtest))
            db.set_section_config("risk", asdict(settings.risk))
            db.set_section_config("ui", asdict(settings.ui))
            db.set_section_config("auth", asdict(settings.auth))
            db.set_section_config("database", asdict(settings.database))
            db.set_section_config("logging", asdict(settings.logging))
            db.set_section_config("notifications", asdict(settings.notifications))
            
        except Exception as e:
            import logging
            logging.getLogger(__name__).warning(f"Failed to sync config to database: {e}")
    
    return settings


def load_config_from_db(db_path: Optional[Path] = None) -> Optional[Settings]:
    """Load configuration from DuckDB database.
    
    Args:
        db_path: Path to the database file.
        
    Returns:
        Settings object if database exists and has config, None otherwise.
    """
    try:
        from src.config.database import get_database
        
        if db_path is None:
            # Try default location
            db_path = Path(__file__).parent.parent.parent / "data" / "traderbot.duckdb"
        
        if not db_path.exists():
            return None
        
        db = get_database(db_path)
        config = db.get_all_config()
        
        if not config:
            return None
        
        # Reconstruct Settings from database config
        ib_cfg = config.get("ib", {})
        app_cfg = config.get("app", {})
        backtest_cfg = config.get("backtest", {})
        risk_cfg = config.get("risk", {})
        ui_cfg = config.get("ui", {})
        auth_cfg = config.get("auth", {})
        database_cfg = config.get("database", {})
        logging_cfg = config.get("logging", {})
        notifications_cfg = config.get("notifications", {})
        
        return Settings(
            ib=IBConfig(**ib_cfg) if ib_cfg else IBConfig(),
            app=AppConfig(**app_cfg) if app_cfg else AppConfig(),
            backtest=BacktestConfig(**backtest_cfg) if backtest_cfg else BacktestConfig(),
            risk=RiskConfig(**risk_cfg) if risk_cfg else RiskConfig(),
            ui=UIConfig(**ui_cfg) if ui_cfg else UIConfig(),
            auth=AuthConfig(**auth_cfg) if auth_cfg else AuthConfig(),
            database=DatabaseConfig(**database_cfg) if database_cfg else DatabaseConfig(),
            logging=LoggingConfig(**logging_cfg) if logging_cfg else LoggingConfig(),
            notifications=NotificationsConfig(**notifications_cfg) if notifications_cfg else NotificationsConfig(),
            _raw_config=config,
        )
        
    except Exception as e:
        import logging
        logging.getLogger(__name__).warning(f"Failed to load config from database: {e}")
        return None


# Global settings instance (lazy loaded)
_settings: Optional[Settings] = None


def get_settings(force_reload: bool = False) -> Settings:
    """Get the global settings instance.
    
    Settings are loaded on first access and cached for subsequent calls.
    
    Args:
        force_reload: Force reloading settings from files.
        
    Returns:
        Settings object.
    """
    global _settings
    if _settings is None or force_reload:
        _settings = load_config()
    return _settings


def update_setting(section: str, key: str, value: Any) -> None:
    """Update a single setting value in the database.
    
    Args:
        section: Configuration section (e.g., 'ib', 'risk')
        key: Setting key within the section
        value: New value
    """
    settings = get_settings()
    if settings.database.enabled:
        try:
            from src.config.database import get_database
            db = get_database()
            db.set_config(section, key, value)
        except Exception as e:
            import logging
            logging.getLogger(__name__).warning(f"Failed to update setting in database: {e}")


# Convenience alias
settings = property(lambda self: get_settings())
