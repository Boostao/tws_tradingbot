"""
Serialization utilities for saving and loading strategies.

Handles JSON serialization/deserialization with proper error handling.
"""

import json
from datetime import datetime
from pathlib import Path
from typing import Optional, List

from .models import Strategy, Rule, Condition, Indicator


class StrategySerializationError(Exception):
    """Raised when strategy serialization/deserialization fails."""
    pass


class StrategyValidationError(Exception):
    """Raised when strategy validation fails."""
    pass


def save_strategy(strategy: Strategy, path: Path) -> None:
    """
    Save a strategy to a JSON file.
    
    Args:
        strategy: The Strategy object to save
        path: Path to save the JSON file
        
    Raises:
        StrategySerializationError: If serialization fails
    """
    try:
        # Update the updated_at timestamp
        strategy.updated_at = datetime.utcnow()
        
        # Convert to dict and serialize
        strategy_dict = strategy.model_dump(mode='json')
        
        # Ensure parent directory exists
        path.parent.mkdir(parents=True, exist_ok=True)
        
        # Write to file with pretty formatting
        with open(path, 'w', encoding='utf-8') as f:
            json.dump(strategy_dict, f, indent=2, ensure_ascii=False)
            
    except Exception as e:
        raise StrategySerializationError(f"Failed to save strategy to {path}: {e}") from e


def load_strategy(path: Path) -> Strategy:
    """
    Load a strategy from a JSON file.
    
    Args:
        path: Path to the JSON file
        
    Returns:
        Strategy object
        
    Raises:
        StrategySerializationError: If loading or parsing fails
        FileNotFoundError: If the file doesn't exist
    """
    if not path.exists():
        raise FileNotFoundError(f"Strategy file not found: {path}")
    
    try:
        with open(path, 'r', encoding='utf-8') as f:
            data = json.load(f)
        
        return Strategy.model_validate(data)
        
    except json.JSONDecodeError as e:
        raise StrategySerializationError(f"Invalid JSON in {path}: {e}") from e
    except Exception as e:
        raise StrategySerializationError(f"Failed to load strategy from {path}: {e}") from e


def validate_strategy_json(json_str: str) -> tuple[bool, Optional[str], Optional[Strategy]]:
    """
    Validate a JSON string as a valid strategy.
    
    Args:
        json_str: JSON string to validate
        
    Returns:
        Tuple of (is_valid, error_message, strategy_object)
        If valid: (True, None, Strategy)
        If invalid: (False, error_message, None)
    """
    try:
        data = json.loads(json_str)
        strategy = Strategy.model_validate(data)
        return True, None, strategy
    except json.JSONDecodeError as e:
        return False, f"Invalid JSON: {e}", None
    except Exception as e:
        return False, f"Validation error: {e}", None


def strategy_to_json(strategy: Strategy, pretty: bool = True) -> str:
    """
    Convert a strategy to a JSON string.
    
    Args:
        strategy: Strategy object to convert
        pretty: Whether to format with indentation
        
    Returns:
        JSON string representation
    """
    strategy_dict = strategy.model_dump(mode='json')
    if pretty:
        return json.dumps(strategy_dict, indent=2, ensure_ascii=False)
    return json.dumps(strategy_dict, ensure_ascii=False)


def strategy_from_json(json_str: str) -> Strategy:
    """
    Create a strategy from a JSON string.
    
    Args:
        json_str: JSON string to parse
        
    Returns:
        Strategy object
        
    Raises:
        StrategySerializationError: If parsing fails
    """
    is_valid, error, strategy = validate_strategy_json(json_str)
    if not is_valid:
        raise StrategySerializationError(error)
    return strategy


def list_strategies(directory: Path) -> List[tuple[str, str, Path]]:
    """
    List all strategy files in a directory.
    
    Args:
        directory: Directory to search
        
    Returns:
        List of (strategy_name, strategy_id, file_path) tuples
    """
    strategies = []
    
    if not directory.exists():
        return strategies
    
    for file_path in directory.glob("*.json"):
        try:
            strategy = load_strategy(file_path)
            strategies.append((strategy.name, strategy.id, file_path))
        except Exception:
            # Skip invalid files
            continue
    
    return sorted(strategies, key=lambda x: x[0])


def copy_strategy(strategy: Strategy, new_name: Optional[str] = None) -> Strategy:
    """
    Create a copy of a strategy with a new ID.
    
    Args:
        strategy: Strategy to copy
        new_name: Optional new name for the copy
        
    Returns:
        New Strategy object
    """
    from uuid import uuid4
    
    # Create a deep copy by serializing and deserializing
    strategy_dict = strategy.model_dump(mode='json')
    
    # Update identifiers
    strategy_dict['id'] = str(uuid4())
    strategy_dict['name'] = new_name or f"{strategy.name} (Copy)"
    strategy_dict['created_at'] = datetime.utcnow().isoformat()
    strategy_dict['updated_at'] = datetime.utcnow().isoformat()
    
    # Update rule IDs
    for rule in strategy_dict.get('rules', []):
        rule['id'] = str(uuid4())
    
    return Strategy.model_validate(strategy_dict)
