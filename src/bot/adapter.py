"""
Interactive Brokers Adapter Module

Provides configuration and factory functions for creating Interactive Brokers
connections using Nautilus Trader's IB integration.

This module handles:
1. IB Gateway/TWS connection configuration
2. Live trading node creation
3. Instrument discovery and caching
"""

import logging
from typing import Dict, List, Optional, Any
from pathlib import Path

# Nautilus Trader imports - wrapped in try/except for environments where not installed
try:
    from nautilus_trader.live.node import TradingNode
    from nautilus_trader.live.config import (
        TradingNodeConfig,
        LiveDataEngineConfig,
        LiveExecEngineConfig,
        LiveRiskEngineConfig,
    )
    from nautilus_trader.adapters.interactive_brokers.config import (
        InteractiveBrokersDataClientConfig,
        InteractiveBrokersExecClientConfig,
        InteractiveBrokersGatewayConfig,
        DockerizedIBGatewayConfig,
        InteractiveBrokersInstrumentProviderConfig,
    )
    from nautilus_trader.adapters.interactive_brokers.factories import (
        InteractiveBrokersLiveDataClientFactory,
        InteractiveBrokersLiveExecClientFactory,
    )
    from nautilus_trader.config import InstrumentProviderConfig
    from nautilus_trader.model.identifiers import TraderId, Venue
    NAUTILUS_IB_AVAILABLE = True
except ImportError:
    NAUTILUS_IB_AVAILABLE = False
    # Stub classes for development without Nautilus
    TradingNode = None
    TradingNodeConfig = None

from src.config.settings import Settings, IBConfig


logger = logging.getLogger(__name__)


class IBConnectionConfig:
    """
    Configuration container for IB connection parameters.
    
    Attributes:
        host: TWS/Gateway host address
        port: TWS/Gateway port (7497 for paper, 7496 for live)
        client_id: Unique client identifier
        account: IB account ID
        timeout: Connection timeout in seconds
        trading_mode: "paper" or "live"
        read_only: If True, only receive data (no trading)
    """
    
    def __init__(
        self,
        host: str = "127.0.0.1",
        port: int = 7497,
        client_id: int = 1,
        account: str = "",
        timeout: int = 30,
        trading_mode: str = "paper",
        read_only: bool = False,
    ):
        self.host = host
        self.port = port
        self.client_id = client_id
        self.account = account
        self.timeout = timeout
        self.trading_mode = trading_mode
        self.read_only = read_only
    
    @classmethod
    def from_settings(cls, settings: Settings) -> "IBConnectionConfig":
        """
        Create config from application settings.
        
        Args:
            settings: Application settings instance
            
        Returns:
            IBConnectionConfig instance
        """
        ib_config = settings.ib
        return cls(
            host=ib_config.host,
            port=ib_config.port,
            client_id=ib_config.client_id,
            account=ib_config.account,
            timeout=ib_config.timeout,
            trading_mode=ib_config.trading_mode,
        )
    
    @classmethod
    def from_ib_config(cls, ib_config: IBConfig) -> "IBConnectionConfig":
        """
        Create config from IBConfig instance.
        
        Args:
            ib_config: IBConfig from settings
            
        Returns:
            IBConnectionConfig instance
        """
        return cls(
            host=ib_config.host,
            port=ib_config.port,
            client_id=ib_config.client_id,
            account=ib_config.account,
            timeout=ib_config.timeout,
            trading_mode=ib_config.trading_mode,
        )
    
    def validate(self) -> List[str]:
        """
        Validate the configuration.
        
        Returns:
            List of validation error messages (empty if valid)
        """
        errors = []
        
        if not self.host:
            errors.append("Host is required")
        
        if self.port not in (7496, 7497, 4001, 4002):
            errors.append(f"Invalid port {self.port}. Use 7497 (TWS paper), 7496 (TWS live), 4002 (Gateway paper), 4001 (Gateway live)")
        
        if self.trading_mode not in ("paper", "live"):
            errors.append(f"Invalid trading mode: {self.trading_mode}")
        
        if self.trading_mode == "live" and self.port in (7497, 4002):
            errors.append("Warning: Trading mode is 'live' but port suggests paper trading")
        
        if self.client_id < 1:
            errors.append("Client ID must be >= 1")
        
        return errors
    
    def __repr__(self) -> str:
        return (
            f"IBConnectionConfig(host={self.host}, port={self.port}, "
            f"client_id={self.client_id}, account={self.account}, "
            f"trading_mode={self.trading_mode})"
        )


def create_ib_gateway_config(connection_config: IBConnectionConfig) -> Optional[Any]:
    """
    Create Nautilus InteractiveBrokersGatewayConfig.
    
    Args:
        connection_config: IB connection configuration
        
    Returns:
        InteractiveBrokersGatewayConfig or None if Nautilus not available
    """
    if not NAUTILUS_IB_AVAILABLE:
        logger.warning("Nautilus IB adapter not available")
        return None
    
    return InteractiveBrokersGatewayConfig(
        host=connection_config.host,
        port=connection_config.port,
        client_id=connection_config.client_id,
        timeout=connection_config.timeout,
    )


def create_ib_data_client_config(
    connection_config: IBConnectionConfig,
    instrument_ids: Optional[List[str]] = None,
) -> Optional[Any]:
    """
    Create Nautilus InteractiveBrokersDataClientConfig.
    
    Args:
        connection_config: IB connection configuration
        instrument_ids: Optional list of instrument IDs to load
        
    Returns:
        InteractiveBrokersDataClientConfig or None if Nautilus not available
    """
    if not NAUTILUS_IB_AVAILABLE:
        logger.warning("Nautilus IB adapter not available")
        return None
    
    gateway_config = create_ib_gateway_config(connection_config)
    
    instrument_provider_config = InteractiveBrokersInstrumentProviderConfig(
        load_ids=frozenset(instrument_ids) if instrument_ids else None,
    )
    
    return InteractiveBrokersDataClientConfig(
        ibg_host=connection_config.host,
        ibg_port=connection_config.port,
        ibg_client_id=connection_config.client_id,
        handle_revised_bars=True,
        instrument_provider=instrument_provider_config,
    )


def create_ib_exec_client_config(
    connection_config: IBConnectionConfig,
    instrument_ids: Optional[List[str]] = None,
) -> Optional[Any]:
    """
    Create Nautilus InteractiveBrokersExecClientConfig.
    
    Args:
        connection_config: IB connection configuration
        instrument_ids: Optional list of instrument IDs for trading
        
    Returns:
        InteractiveBrokersExecClientConfig or None if Nautilus not available
    """
    if not NAUTILUS_IB_AVAILABLE:
        logger.warning("Nautilus IB adapter not available")
        return None
    
    instrument_provider_config = InteractiveBrokersInstrumentProviderConfig(
        load_ids=frozenset(instrument_ids) if instrument_ids else None,
    )
    
    return InteractiveBrokersExecClientConfig(
        ibg_host=connection_config.host,
        ibg_port=connection_config.port,
        ibg_client_id=connection_config.client_id,
        account_id=connection_config.account,
        instrument_provider=instrument_provider_config,
    )


def create_live_node(
    connection_config: IBConnectionConfig,
    strategy: Any,
    instruments: Optional[List[str]] = None,
    trader_id: str = "TRADER-001",
) -> Optional[Any]:
    """
    Create a Nautilus TradingNode configured for Interactive Brokers.
    
    This is the main entry point for creating a live trading node with
    the IB adapter.
    
    Args:
        connection_config: IB connection configuration
        strategy: The trading strategy to run (NautilusDynamicRuleStrategy)
        instruments: List of instrument IDs to trade
        trader_id: Unique trader identifier
        
    Returns:
        TradingNode or None if Nautilus not available
        
    Example:
        config = IBConnectionConfig.from_settings(settings)
        strategy = NautilusDynamicRuleStrategy(strategy_config)
        node = create_live_node(config, strategy, ["SPY.ARCA"])
        node.run()
    """
    if not NAUTILUS_IB_AVAILABLE:
        logger.error("Nautilus IB adapter not available. Install with: pip install nautilus_trader[ib]")
        return None
    
    # Validate connection config
    errors = connection_config.validate()
    if errors:
        for error in errors:
            logger.error(f"Config error: {error}")
        return None
    
    instruments = instruments or ["SPY.ARCA"]
    
    # Create data client config
    data_config = create_ib_data_client_config(connection_config, instruments)
    
    # Create exec client config (if not read-only)
    exec_config = None
    if not connection_config.read_only:
        exec_config = create_ib_exec_client_config(connection_config, instruments)
    
    # Build trading node config
    node_config = TradingNodeConfig(
        trader_id=TraderId(trader_id),
        data_clients={
            "IB": data_config,
        },
        exec_clients={
            "IB": exec_config,
        } if exec_config else {},
        data_engine=LiveDataEngineConfig(
            time_bars_build_with_no_updates=True,
            validate_data_sequence=True,
        ),
        exec_engine=LiveExecEngineConfig(
            reconciliation=True,
        ),
        risk_engine=LiveRiskEngineConfig(),
    )
    
    # Create the trading node
    node = TradingNode(config=node_config)
    
    # Add data client factory
    node.add_data_client_factory("IB", InteractiveBrokersLiveDataClientFactory)
    
    # Add exec client factory (if not read-only)
    if not connection_config.read_only:
        node.add_exec_client_factory("IB", InteractiveBrokersLiveExecClientFactory)
    
    # Build the node
    node.build()
    
    # Add the strategy
    node.trader.add_strategy(strategy)
    
    logger.info(f"Created TradingNode: trader_id={trader_id}, instruments={instruments}")
    
    return node


def check_ib_connection(
    host: str = "127.0.0.1",
    port: int = 7497,
    client_id: int = 99,
    timeout: int = 5,
) -> Dict[str, Any]:
    """
    Check if TWS/IB Gateway is running and accepting connections.
    
    This is a lightweight connection test that doesn't require
    the full Nautilus setup.
    
    Args:
        host: TWS/Gateway host
        port: TWS/Gateway port
        client_id: Client ID for test connection
        timeout: Connection timeout in seconds
        
    Returns:
        Dict with connection status:
        {
            "connected": bool,
            "message": str,
            "host": str,
            "port": int,
        }
    """
    import socket
    
    result = {
        "connected": False,
        "message": "",
        "host": host,
        "port": port,
    }
    
    try:
        # Try to connect to the socket
        sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        sock.settimeout(timeout)
        sock.connect((host, port))
        sock.close()
        
        result["connected"] = True
        result["message"] = f"Successfully connected to {host}:{port}"
        logger.info(result["message"])
        
    except socket.timeout:
        result["message"] = f"Connection timeout to {host}:{port}. Is TWS/Gateway running?"
        logger.warning(result["message"])
        
    except ConnectionRefusedError:
        result["message"] = f"Connection refused at {host}:{port}. Start TWS/Gateway and enable API connections."
        logger.warning(result["message"])
        
    except Exception as e:
        result["message"] = f"Connection error: {str(e)}"
        logger.error(result["message"])
    
    return result


def get_available_ports() -> Dict[str, int]:
    """
    Get standard IB connection ports.
    
    Returns:
        Dict mapping mode to port number
    """
    return {
        "tws_paper": 7497,
        "tws_live": 7496,
        "gateway_paper": 4002,
        "gateway_live": 4001,
    }


class IBAdapter:
    """
    High-level adapter for Interactive Brokers integration.
    
    Provides a simplified interface for:
    - Connection management
    - Instrument discovery
    - Live node creation
    
    Example:
        adapter = IBAdapter(settings)
        if adapter.check_connection():
            node = adapter.create_trading_node(strategy, ["SPY.ARCA"])
            node.run()
    """
    
    def __init__(self, settings: Optional[Settings] = None):
        """
        Initialize the IB adapter.
        
        Args:
            settings: Application settings (loads from default if None)
        """
        if settings is None:
            settings = Settings()
        
        self.settings = settings
        self.connection_config = IBConnectionConfig.from_settings(settings)
        self._node: Optional[Any] = None
        
        logger.info(f"IBAdapter initialized: {self.connection_config}")
    
    def check_connection(self) -> bool:
        """
        Check if TWS/Gateway is available.
        
        Returns:
            True if connection successful
        """
        result = check_ib_connection(
            host=self.connection_config.host,
            port=self.connection_config.port,
            timeout=self.connection_config.timeout,
        )
        return result["connected"]
    
    def create_trading_node(
        self,
        strategy: Any,
        instruments: Optional[List[str]] = None,
        trader_id: str = "TRADER-001",
    ) -> Optional[Any]:
        """
        Create a live trading node.
        
        Args:
            strategy: Trading strategy instance
            instruments: List of instruments to trade
            trader_id: Unique trader ID
            
        Returns:
            TradingNode or None
        """
        self._node = create_live_node(
            connection_config=self.connection_config,
            strategy=strategy,
            instruments=instruments,
            trader_id=trader_id,
        )
        return self._node
    
    def get_node(self) -> Optional[Any]:
        """Get the current trading node."""
        return self._node
    
    @property
    def is_paper_trading(self) -> bool:
        """Check if configured for paper trading."""
        return self.connection_config.trading_mode == "paper"
    
    @property
    def is_live_trading(self) -> bool:
        """Check if configured for live trading."""
        return self.connection_config.trading_mode == "live"
