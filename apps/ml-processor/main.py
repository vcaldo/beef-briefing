#!/usr/bin/env python3
"""
ML Processor - Analyzes Portuguese chat messages using local ML models.

Usage:
    python main.py                      # Run continuous processing (dev)
    python main.py --once               # Run single batch and exit
    python main.py --status             # Print processing status
    python main.py --api-url URL        # Override API endpoint
    python main.py --api-key-file PATH  # Override API key file
"""

import os
import sys

# Load config first (before New Relic import)
from config import load_config

_config = load_config()

# Initialize New Relic APM if configured (must be done before other imports)
if _config.new_relic_enabled():
    os.environ["NEW_RELIC_APP_NAME"] = _config.new_relic_full_app_name
    os.environ["NEW_RELIC_LICENSE_KEY"] = _config.new_relic_license_key
    import newrelic.agent

    newrelic.agent.initialize()

# Now import everything else
import argparse
import logging

from src.pipeline.processor import MLProcessor


def setup_logging(config):
    """Configure logging based on environment."""
    level_map = {
        "debug": logging.DEBUG,
        "info": logging.INFO,
        "warn": logging.WARNING,
        "warning": logging.WARNING,
        "error": logging.ERROR,
    }
    level = level_map.get(config.log_level.lower(), logging.INFO)

    if config.is_production():
        # JSON format for production
        format_str = (
            '{"time":"%(asctime)s","level":"%(levelname)s",'
            '"logger":"%(name)s","message":"%(message)s"}'
        )
    else:
        # Human-readable format for development
        format_str = "%(asctime)s - %(name)s - %(levelname)s - %(message)s"

    logging.basicConfig(
        level=level,
        format=format_str,
        stream=sys.stdout,
    )

    # Suppress verbose logging from libraries
    logging.getLogger("httpx").setLevel(logging.WARNING)
    logging.getLogger("transformers").setLevel(logging.WARNING)
    logging.getLogger("sentence_transformers").setLevel(logging.WARNING)

    return logging.getLogger(__name__)


def main():
    parser = argparse.ArgumentParser(
        description="ML Processor for Portuguese chat analysis"
    )
    parser.add_argument(
        "--once",
        action="store_true",
        help="Run single batch and exit",
    )
    parser.add_argument(
        "--status",
        action="store_true",
        help="Print processing status and exit",
    )
    parser.add_argument(
        "--limit",
        type=int,
        help="Override batch size limit",
    )
    parser.add_argument(
        "--api-url",
        type=str,
        help="Override API service URL (e.g., https://api.example.com)",
    )
    parser.add_argument(
        "--api-key-file",
        type=str,
        help="Override API key file path",
    )
    args = parser.parse_args()

    # Use module-level config (already loaded for New Relic init)
    config = _config

    # Override config from CLI args
    if args.limit:
        config.batch_size = args.limit
    if args.api_url:
        config.api_service_url = args.api_url
    if args.api_key_file:
        config.api_key_file = args.api_key_file

    # Setup logging
    logger = setup_logging(config)

    logger.info(f"ML Processor starting (device: {config.device})")
    logger.info(f"API Service: {config.api_service_url}")
    logger.info(f"Qdrant: {config.qdrant_host}:{config.qdrant_port}")
    if config.new_relic_enabled():
        logger.info(f"New Relic APM: {config.new_relic_full_app_name}")

    # Create processor
    processor = MLProcessor(config)

    try:
        # Setup connections and load models
        processor.setup()

        if args.status:
            # Just print status and exit
            processor.print_status()
            return

        if args.once:
            # Run single batch
            processed = processor.run_once()
            logger.info(f"Processed {processed} messages")
        else:
            # Run continuous loop
            processor.run_continuous()

    except KeyboardInterrupt:
        logger.info("Interrupted by user")

    except Exception as e:
        logger.error(f"Fatal error: {e}", exc_info=True)
        sys.exit(1)

    finally:
        processor.cleanup()

    logger.info("ML Processor stopped")


if __name__ == "__main__":
    main()
