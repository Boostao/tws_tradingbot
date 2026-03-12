"""Configuration management for the Trading Bot.

Loads settings from YAML files and environment variables,
providing typed dataclasses for easy access.
"""

import os
import yaml
from dataclasses import dataclass, field
from pathlib import Path
from typing import Dict, Any, Optional
from dotenv import load_dotenv


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
            "LOG_LEVEL": ("app", "log_level"),
            "APP_ACTIVE_STRATEGY_PATH": ("app", "active_strategy_path"),
            "APP_WATCHLIST_PATH": ("app", "watchlist_path"),
            "APP_SYMBOL_CACHE_PATH": ("app", "symbol_cache_path"),
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
        app_cfg = raw_config.get("app", {})
        logging_cfg = raw_config.get("logging", {})

        return Settings(
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


def update_setting(section: str, key: str, value: Any) -> None:
    """Update a single setting value in the in-memory cached configuration."""
    settings = get_settings()
    section_obj = getattr(settings, section, None)
    if section_obj is not None and hasattr(section_obj, key):
        setattr(section_obj, key, value)


# Convenience alias
settings = property(lambda self: get_settings())
