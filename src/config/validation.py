from typing import Dict, Any, List
import re


class ConfigValidationError(Exception):
    """Exception raised for configuration validation errors."""
    pass


def validate_config(config: Dict[str, Any]) -> None:
    """Validate the configuration values."""
    errors = []

    # Validate TWS settings
    errors.extend(_validate_tws_config(config.get('tws', {})))

    # Validate bot settings
    errors.extend(_validate_bot_config(config.get('bot', {})))

    # Validate risk settings
    errors.extend(_validate_risk_config(config.get('risk', {})))

    # Validate UI settings
    errors.extend(_validate_ui_config(config.get('ui', {})))

    # Validate logging settings
    errors.extend(_validate_logging_config(config.get('logging', {})))

    # Validate data settings
    errors.extend(_validate_data_config(config.get('data', {})))

    if errors:
        raise ConfigValidationError(f"Configuration validation failed:\n" + "\n".join(errors))


def _validate_tws_config(tws: Dict[str, Any]) -> List[str]:
    """Validate TWS configuration."""
    errors = []

    # Host
    if 'host' in tws:
        if not isinstance(tws['host'], str):
            errors.append("tws.host must be a string")
        elif not _is_valid_ip_or_hostname(tws['host']):
            errors.append("tws.host must be a valid IP address or hostname")

    # Port
    if 'port' in tws:
        if not isinstance(tws['port'], int) or not (1024 <= tws['port'] <= 65535):
            errors.append("tws.port must be an integer between 1024 and 65535")

    # Client ID
    if 'client_id' in tws:
        if not isinstance(tws['client_id'], int) or tws['client_id'] < 0:
            errors.append("tws.client_id must be a non-negative integer")

    # Timeout
    if 'timeout' in tws:
        if not isinstance(tws['timeout'], (int, float)) or tws['timeout'] <= 0:
            errors.append("tws.timeout must be a positive number")

    return errors


def _validate_bot_config(bot: Dict[str, Any]) -> List[str]:
    """Validate bot configuration."""
    errors = []

    # Starting capital
    if 'starting_capital' in bot:
        if not isinstance(bot['starting_capital'], (int, float)) or bot['starting_capital'] <= 0:
            errors.append("bot.starting_capital must be a positive number")

    # Max positions
    if 'max_positions' in bot:
        if not isinstance(bot['max_positions'], int) or bot['max_positions'] <= 0:
            errors.append("bot.max_positions must be a positive integer")

    # Position size fraction
    if 'position_size_fraction' in bot:
        if not isinstance(bot['position_size_fraction'], (int, float)) or not (0 < bot['position_size_fraction'] <= 1):
            errors.append("bot.position_size_fraction must be a number between 0 and 1")

    # Loop interval
    if 'loop_interval' in bot:
        if not isinstance(bot['loop_interval'], (int, float)) or bot['loop_interval'] <= 0:
            errors.append("bot.loop_interval must be a positive number")

    # Market times
    for time_key in ['market_open_time', 'market_close_time']:
        if time_key in bot:
            if not isinstance(bot[time_key], str) or not re.match(r'^\d{2}:\d{2}$', bot[time_key]):
                errors.append(f"bot.{time_key} must be in HH:MM format")

    return errors


def _validate_risk_config(risk: Dict[str, Any]) -> List[str]:
    """Validate risk configuration."""
    errors = []

    # Percentages
    for key in ['max_drawdown', 'daily_loss_limit', 'stop_loss_percentage']:
        if key in risk:
            if not isinstance(risk[key], (int, float)) or not (0 <= risk[key] <= 1):
                errors.append(f"risk.{key} must be a number between 0 and 1")

    # VIX threshold
    if 'vix_threshold' in risk:
        if not isinstance(risk['vix_threshold'], (int, float)) or risk['vix_threshold'] <= 0:
            errors.append("risk.vix_threshold must be a positive number")

    return errors


def _validate_ui_config(ui: Dict[str, Any]) -> List[str]:
    """Validate UI configuration."""
    errors = []

    # Port
    if 'port' in ui:
        if not isinstance(ui['port'], int) or not (1024 <= ui['port'] <= 65535):
            errors.append("ui.port must be an integer between 1024 and 65535")

    # Host
    if 'host' in ui:
        if not isinstance(ui['host'], str):
            errors.append("ui.host must be a string")

    # Refresh interval
    if 'refresh_interval' in ui:
        if not isinstance(ui['refresh_interval'], (int, float)) or ui['refresh_interval'] <= 0:
            errors.append("ui.refresh_interval must be a positive number")

    return errors


def _validate_logging_config(logging: Dict[str, Any]) -> List[str]:
    """Validate logging configuration."""
    errors = []

    # Level
    if 'level' in logging:
        valid_levels = ['DEBUG', 'INFO', 'WARNING', 'ERROR', 'CRITICAL']
        if not isinstance(logging['level'], str) or logging['level'].upper() not in valid_levels:
            errors.append(f"logging.level must be one of: {', '.join(valid_levels)}")

    # Format
    if 'format' in logging:
        if not isinstance(logging['format'], str):
            errors.append("logging.format must be a string")

    # File configuration
    if 'file' in logging:
        file_config = logging['file']
        if isinstance(file_config, dict):
            # Validate file.enabled
            if 'enabled' in file_config and not isinstance(file_config['enabled'], bool):
                errors.append("logging.file.enabled must be a boolean")

            # Validate file.path
            if 'path' in file_config and not isinstance(file_config['path'], str):
                errors.append("logging.file.path must be a string")

            # Validate file.max_bytes
            if 'max_bytes' in file_config:
                if not isinstance(file_config['max_bytes'], int) or file_config['max_bytes'] <= 0:
                    errors.append("logging.file.max_bytes must be a positive integer")

            # Validate file.backup_count
            if 'backup_count' in file_config:
                if not isinstance(file_config['backup_count'], int) or file_config['backup_count'] < 0:
                    errors.append("logging.file.backup_count must be a non-negative integer")
        elif isinstance(file_config, str):
            # Backward compatibility: file as string
            pass
        else:
            errors.append("logging.file must be a string or a dictionary")

    # Console configuration
    if 'console' in logging:
        console_config = logging['console']
        if isinstance(console_config, dict):
            # Validate console.enabled
            if 'enabled' in console_config and not isinstance(console_config['enabled'], bool):
                errors.append("logging.console.enabled must be a boolean")

            # Validate console.level
            if 'level' in console_config:
                if not isinstance(console_config['level'], str) or console_config['level'].upper() not in valid_levels:
                    errors.append(f"logging.console.level must be one of: {', '.join(valid_levels)}")
        else:
            errors.append("logging.console must be a dictionary")

    return errors


def _validate_data_config(data: Dict[str, Any]) -> List[str]:
    """Validate data configuration."""
    errors = []

    # Cache timeout
    if 'cache_timeout' in data:
        if not isinstance(data['cache_timeout'], (int, float)) or data['cache_timeout'] <= 0:
            errors.append("data.cache_timeout must be a positive number")

    # Max API calls
    if 'max_api_calls_per_minute' in data:
        if not isinstance(data['max_api_calls_per_minute'], (int, float)) or data['max_api_calls_per_minute'] <= 0:
            errors.append("data.max_api_calls_per_minute must be a positive number")

    return errors


def _is_valid_ip_or_hostname(value: str) -> bool:
    """Check if value is a valid IP address or hostname."""
    # Simple check for IP or hostname
    import ipaddress
    try:
        ipaddress.ip_address(value)
        return True
    except ValueError:
        pass
    # Basic hostname check
    if re.match(r'^[a-zA-Z0-9.-]+$', value) and len(value) <= 253:
        return True
    return False