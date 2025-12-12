"""
Entry point for Beef Dashboard.
"""

import logging
import sys
from typing import Optional

from src.config import Config, load_config, setup_logging
from src.app import create_app

logger = logging.getLogger(__name__)

# Global app instance for gunicorn
app: Optional["dash.Dash"] = None
server = None


def init_new_relic(config: Config) -> None:
    """Initialize New Relic APM if configured."""
    if not config.new_relic_enabled():
        logger.debug("New Relic not configured, skipping initialization")
        return

    try:
        import newrelic.agent

        app_name = f"{config.new_relic_app_name}-dashboard-{config.environment}"
        newrelic.agent.initialize(
            license_key=config.new_relic_license_key,
            app_name=app_name,
            log_level="info" if config.is_production() else "debug",
        )
        logger.info(f"New Relic initialized: {app_name}")
    except ImportError:
        logger.warning("newrelic package not installed, skipping APM initialization")
    except Exception as e:
        logger.warning(f"Failed to initialize New Relic: {e}")


def main() -> None:
    """Main entry point for local development."""
    try:
        # Load configuration
        config = load_config()
    except Exception as e:
        print(f"Failed to load configuration: {e}", file=sys.stderr)
        sys.exit(1)

    # Setup logging
    setup_logging(config)

    logger.info(
        "Starting Beef Dashboard",
        extra={
            "port": config.dashboard_port,
            "environment": config.environment,
            "allowed_chats": len(config.allowed_chat_ids),
        }
    )

    # Initialize New Relic
    init_new_relic(config)

    # Create and run the app
    dash_app = create_app(config)

    # Run in development mode
    dash_app.run(
        host="0.0.0.0",
        port=config.dashboard_port,
        debug=config.is_development(),
    )


def create_server():
    """
    Create the WSGI server for production deployment with gunicorn.

    Usage: gunicorn --bind 0.0.0.0:8050 src.main:server
    """
    global app, server

    try:
        config = load_config()
    except Exception as e:
        print(f"Failed to load configuration: {e}", file=sys.stderr)
        sys.exit(1)

    setup_logging(config)

    logger.info(
        "Initializing Beef Dashboard for production",
        extra={
            "environment": config.environment,
            "allowed_chats": len(config.allowed_chat_ids),
        }
    )

    init_new_relic(config)

    app = create_app(config)
    server = app.server

    return server


# Initialize server for gunicorn imports
# This runs when the module is imported by gunicorn
try:
    _config = load_config()
    setup_logging(_config)
    init_new_relic(_config)
    app = create_app(_config)
    server = app.server
except Exception as e:
    # Log error but don't crash - allows import for testing
    logging.error(f"Failed to initialize app on import: {e}")
    server = None


if __name__ == "__main__":
    main()
