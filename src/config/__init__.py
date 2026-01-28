"""Configuration module for the Trading Bot."""

from .settings import (
    Settings,
    IBConfig,
    AppConfig,
    BacktestConfig,
    RiskConfig,
    UIConfig,
    DatabaseConfig,
    LoggingConfig,
    load_config,
    load_config_from_db,
    get_settings,
    update_setting,
)

from .database import (
    DatabaseManager,
    get_database,
    reset_database_instance,
)

__all__ = [
    'Settings',
    'IBConfig',
    'AppConfig',
    'BacktestConfig',
    'RiskConfig',
    'UIConfig',
    'DatabaseConfig',
    'LoggingConfig',
    'load_config',
    'load_config_from_db',
    'get_settings',
    'update_setting',
    'DatabaseManager',
    'get_database',
    'reset_database_instance',
]