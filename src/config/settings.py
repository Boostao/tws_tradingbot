"""Configuration management for the Trading Bot.

Loads settings from YAML files and environment variables,
providing typed dataclasses for easy access.
"""

import os
import yaml
from dataclasses import asdict, dataclass, field
from pathlib import Path
from typing import Dict, Any, Optional
from dotenv import load_dotenv


@dataclass
class IBConfig:
    """Interactive Brokers TWS connection configuration."""

    host: str = "127.0.0.1"
    port: int = 7497
    client_id: int = 1
    account: str = ""
    timeout: int = 5
    trading_mode: str = "paper"


@dataclass
class RuntimeConfig:
    """Trading runtime configuration exposed to the control sidebar."""

    fixed_notional: float = 10000.0
    bracket_enabled: bool = False
    stop_loss_pct: float = 2.0
    take_profit_pct: float = 4.0


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
class Settings:
    """Main settings container with all configuration sections."""
    ib: IBConfig = field(default_factory=IBConfig)
    runtime: RuntimeConfig = field(default_factory=RuntimeConfig)
    app: AppConfig = field(default_factory=AppConfig)
    logging: LoggingConfig = field(default_factory=LoggingConfig)

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
            "IB_TIMEOUT": ("ib", "timeout"),
            "IB_TRADING_MODE": ("ib", "trading_mode"),
            "RUNTIME_FIXED_NOTIONAL": ("runtime", "fixed_notional"),
            "RUNTIME_BRACKET_ENABLED": ("runtime", "bracket_enabled"),
            "RUNTIME_STOP_LOSS_PCT": ("runtime", "stop_loss_pct"),
            "RUNTIME_TAKE_PROFIT_PCT": ("runtime", "take_profit_pct"),
            "APP_ACTIVE_STRATEGY_PATH": ("app", "active_strategy_path"),
            "APP_WATCHLIST_PATH": ("app", "watchlist_path"),
            "APP_SYMBOL_CACHE_PATH": ("app", "symbol_cache_path"),
        }

        for env_key, config_path in env_mappings.items():
            value = os.getenv(env_key)
            if value is not None:
                self._set_nested(config, config_path, self._convert_value(value))

        log_level = os.getenv("LOG_LEVEL")
        if log_level is not None:
            converted = self._convert_value(log_level)
            self._set_nested(config, ("app", "log_level"), converted)
            self._set_nested(config, ("logging", "level"), converted)
            self._set_nested(config, ("logging", "console", "level"), converted)

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
        runtime_cfg = raw_config.get("runtime", {})
        app_cfg = raw_config.get("app", {})
        logging_cfg = raw_config.get("logging", {})

        return Settings(
            ib=IBConfig(
                host=ib_cfg.get("host", "127.0.0.1"),
                port=ib_cfg.get("port", 7497),
                client_id=ib_cfg.get("client_id", 1),
                account=ib_cfg.get("account", ""),
                timeout=ib_cfg.get("timeout", 5),
                trading_mode=ib_cfg.get("trading_mode", "paper"),
            ),
            runtime=RuntimeConfig(
                fixed_notional=runtime_cfg.get("fixed_notional", 10000.0),
                bracket_enabled=runtime_cfg.get("bracket_enabled", False),
                stop_loss_pct=runtime_cfg.get("stop_loss_pct", 2.0),
                take_profit_pct=runtime_cfg.get("take_profit_pct", 4.0),
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
            _raw_config=raw_config,
        )


def load_config(config_dir: Optional[Path] = None) -> Settings:
    """Load configuration and return Settings object.

    Args:
        config_dir: Optional path to config directory. Defaults to project's config/ folder.

    Returns:
        Settings object with all configuration values.
    """
    loader = ConfigLoader(config_dir)
    return loader.load()


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


def _apply_setting_update(settings: Settings, section: str, key: str, value: Any) -> None:
    section_obj = getattr(settings, section, None)
    if section_obj is not None and hasattr(section_obj, key):
        setattr(section_obj, key, value)

    if section not in settings._raw_config or not isinstance(settings._raw_config.get(section), dict):
        settings._raw_config[section] = {}
    settings._raw_config[section][key] = value


def _persist_settings(settings: Settings) -> None:
    config_dir = Path(os.getenv("TRADERBOT_CONFIG_DIR", Path(__file__).parent.parent.parent / "config"))
    config_dir.mkdir(parents=True, exist_ok=True)
    default_config_path = config_dir / "default.yaml"
    with open(default_config_path, "w") as config_file:
        yaml.safe_dump(settings._raw_config, config_file, sort_keys=False)


def update_settings(updates: Dict[str, Dict[str, Any]]) -> None:
    """Update multiple setting values and persist them with a single file write."""
    settings = get_settings()
    for section, values in updates.items():
        if not isinstance(values, dict):
            continue
        for key, value in values.items():
            _apply_setting_update(settings, section, key, value)
    _persist_settings(settings)


def update_setting(section: str, key: str, value: Any) -> None:
    """Update a setting value in memory and persist it to the default config file."""
    update_settings({section: {key: value}})


def get_redacted_settings() -> Dict[str, Any]:
    """Return the subset of settings that is safe and useful for the UI."""
    settings = get_settings()
    return {
        "ib": asdict(settings.ib),
        "runtime": asdict(settings.runtime),
    }


# Convenience alias
settings = property(lambda self: get_settings())
