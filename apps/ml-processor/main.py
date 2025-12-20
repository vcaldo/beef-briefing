#!/usr/bin/env python3
"""
ML Processor - Analyzes Portuguese chat messages using local ML models.

Usage:
    python main.py process --chat-id -1003280306634 [--limit N] [--batch-size N]
    python main.py process --chat-id -1003280306634 --from-date 2024-11-01 --to-date 2024-12-01
    python main.py status --chat-id -1003280306634
    python main.py continuous --chat-id -1003280306634
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
    # For non-web apps, explicitly register and wait for connection
    newrelic.agent.register_application(timeout=10.0)

# Now import everything else
import argparse
import logging
from datetime import datetime

from sqlalchemy import create_engine

from src.analyzers.base import AnalyzerRegistry
from src.database import MLQueries
from src.instrumentation import (
    add_custom_attributes,
    background_task,
    record_custom_metric,
)
from src.processor import MLProcessor

logger = logging.getLogger(__name__)


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
    logging.getLogger("qdrant_client").setLevel(logging.WARNING)

    return logging.getLogger(__name__)


def create_processor(config) -> MLProcessor:
    """Create and setup the ML processor."""
    # Create database engine
    engine = create_engine(config.dsn(), pool_pre_ping=True)

    # Create components
    queries = MLQueries(engine)
    registry = AnalyzerRegistry(config)
    processor = MLProcessor(config, queries, registry)

    return processor


def parse_date(date_str: str | None) -> datetime | None:
    """Parse date string in YYYY-MM-DD format."""
    if date_str is None:
        return None
    try:
        return datetime.strptime(date_str, "%Y-%m-%d")
    except ValueError:
        raise ValueError(f"Invalid date format: {date_str}. Use YYYY-MM-DD")


def run_process(args, config):
    """Run batch processing command."""
    logger.info(f"Processing chat {args.chat_id}")

    # Use config batch_size if not specified via CLI
    batch_size = args.batch_size if args.batch_size is not None else config.batch_size

    # Parse date arguments
    from_date = parse_date(getattr(args, "from_date", None))
    to_date = parse_date(getattr(args, "to_date", None))

    # Add custom attributes for APM tracking
    add_custom_attributes(
        {
            "chat_id": args.chat_id,
            "batch_size": batch_size,
            "limit": args.limit,
            "from_date": str(from_date.date()) if from_date else None,
            "to_date": str(to_date.date()) if to_date else None,
        }
    )

    if from_date:
        logger.info(f"From date: {from_date.date()}")
    if to_date:
        logger.info(f"To date: {to_date.date()}")

    logger.info(f"Batch size: {batch_size}")

    processor = create_processor(config)
    processor.setup()

    try:
        # Log configured providers
        logger.info(f"Sentiment provider: {config.sentiment_provider}")
        logger.info(f"Toxicity provider: {config.toxicity_provider}")
        logger.info(f"Topics provider: {config.topics_provider}")
        logger.info(f"NER provider: {config.ner_provider}")

        # Process in batches
        total_processed = 0
        remaining = args.limit

        while True:
            # Determine batch size for this iteration
            current_batch = batch_size
            if remaining is not None:
                current_batch = min(batch_size, remaining)
                if current_batch <= 0:
                    break

            # Process batch
            count = processor.process_batch(
                args.chat_id,
                limit=current_batch,
                from_date=from_date,
                to_date=to_date,
            )
            total_processed += count

            # Record batch metrics
            record_custom_metric("Custom/MLProcessor/BatchSize", count)

            logger.info(f"Batch complete: {count} messages (total: {total_processed})")

            # Update remaining
            if remaining is not None:
                remaining -= count

            # No more messages to process
            if count < current_batch:
                break

        # Record total processed metric
        record_custom_metric("Custom/MLProcessor/TotalProcessed", total_processed)
        logger.info(f"Processing complete: {total_processed} total messages processed")
        return total_processed

    finally:
        processor.cleanup()


@background_task(name="status", group="MLProcessor")
def run_status(args, config):
    """Run status command."""
    add_custom_attributes({"chat_id": args.chat_id})

    processor = create_processor(config)
    processor.setup()

    try:
        processor.print_status(args.chat_id)
    finally:
        processor.cleanup()


@background_task(name="generate-cards", group="MLProcessor")
def run_generate_cards(args, config):
    """Run card generation command."""
    from zoneinfo import ZoneInfo, ZoneInfoNotFoundError

    logger.info(f"Generating cards for chat {args.chat_id}")

    # Validate timezone
    try:
        ZoneInfo(args.timezone)
    except ZoneInfoNotFoundError:
        logger.error(f"Invalid timezone: {args.timezone}")
        raise ValueError(f"Invalid timezone: {args.timezone}. Use IANA format (e.g., America/Sao_Paulo)")

    # Parse date arguments
    week_start = parse_date(args.week) if args.week else None
    from_date = parse_date(getattr(args, "from_date", None))
    to_date = parse_date(getattr(args, "to_date", None))

    add_custom_attributes(
        {
            "chat_id": args.chat_id,
            "week": args.week,
            "window_days": args.window_days,
            "min_messages": args.min_messages,
            "timezone": args.timezone,
            "from_date": str(from_date.date()) if from_date else None,
            "to_date": str(to_date.date()) if to_date else None,
        }
    )

    logger.info(f"Timezone: {args.timezone}")
    if from_date:
        logger.info(f"From date: {from_date.date()}")
    if to_date:
        logger.info(f"To date: {to_date.date()}")

    # Create database engine
    engine = create_engine(config.dsn(), pool_pre_ping=True)

    # Create generator and run
    from src.cards import CardGenerator

    generator = CardGenerator(engine, timezone=args.timezone, window_days=args.window_days)
    result = generator.generate_cards(
        chat_id=args.chat_id,
        week_start=week_start,
        min_messages=args.min_messages,
        from_date=from_date,
        to_date=to_date,
    )

    # Print results
    print("\nCard Generation Results:")
    print(f"  Timezone: {args.timezone}")
    print(f"  Week: {result['week_start']} - {result['week_end']}")
    print(f"  Stats window: {result['window_start']} - {result['window_end']}")
    print(f"  Active users: {result['active_users']}")
    print(f"  Cards generated: {result['generated']}")
    print(f"  Users skipped: {result['skipped']}")

    return result


@background_task(name="render-cards", group="MLProcessor")
def run_render_cards(args, config):
    """Run card image rendering command."""
    logger.info(f"Rendering card images for chat {args.chat_id}")

    # Validate config
    if not config.card_image_generator_url:
        logger.error("CARD_IMAGE_GENERATOR_URL not configured")
        print("\nError: Card image generator service not configured.")
        print("Set CARD_IMAGE_GENERATOR_URL and CARD_IMAGE_GENERATOR_API_KEY environment variables.")
        return None

    if not config.card_image_generator_api_key:
        logger.error("CARD_IMAGE_GENERATOR_API_KEY not configured")
        print("\nError: Card image generator API key not configured.")
        print("Set CARD_IMAGE_GENERATOR_API_KEY environment variable.")
        return None

    add_custom_attributes(
        {
            "chat_id": args.chat_id,
            "week": args.week,
            "theme": args.theme,
            "force": args.force,
        }
    )

    # Create client
    from src.cards import CardImageClient

    client = CardImageClient(
        base_url=config.card_image_generator_url,
        api_key=config.card_image_generator_api_key,
        timeout=config.card_image_generator_timeout,
    )

    # Check health
    if not client.health_check():
        logger.error("Card image generator service is not healthy")
        print("\nError: Card image generator service is not responding.")
        print(f"Check if the service is running at {config.card_image_generator_url}")
        return None

    # Determine week (use provided or let service figure it out)
    week_start = args.week if args.week else ""

    # If no week specified, we need to get the latest from the database
    if not week_start:
        from sqlalchemy import create_engine, text

        engine = create_engine(config.dsn(), pool_pre_ping=True)
        with engine.connect() as conn:
            result = conn.execute(
                text(
                    "SELECT week_start FROM ml_user_cards WHERE chat_id = :chat_id "
                    "ORDER BY week_start DESC LIMIT 1"
                ),
                {"chat_id": args.chat_id},
            ).scalar()
            if result:
                week_start = result.isoformat() if hasattr(result, "isoformat") else str(result)
            else:
                logger.error("No cards found for this chat")
                print("\nError: No cards found for this chat. Run generate-cards first.")
                return None

    logger.info(f"Rendering images for week: {week_start}")

    # Render cards
    result = client.render_cards(
        chat_id=args.chat_id,
        week_start=week_start,
        theme=args.theme,
        force_regenerate=args.force,
    )

    # Print results
    print("\nCard Image Rendering Results:")
    print(f"  Week: {week_start}")
    print(f"  Theme: {args.theme}")
    print(f"  Generated: {result.generated}")
    print(f"  Skipped: {result.skipped}")
    print(f"  Failed: {result.failed}")

    if result.failed > 0:
        print("\nFailed renders:")
        for r in result.results:
            if r.get("status") == "failed":
                print(f"  - User {r.get('user_id')}: {r.get('error')}")

    return result


def run_continuous(args, config):
    """Run continuous processing command."""
    logger.info(f"Starting continuous processing for chat {args.chat_id}")

    add_custom_attributes({"chat_id": args.chat_id})

    processor = create_processor(config)
    processor.setup()

    try:
        # Log configured providers
        logger.info(f"Sentiment provider: {config.sentiment_provider}")
        logger.info(f"Toxicity provider: {config.toxicity_provider}")
        logger.info(f"Topics provider: {config.topics_provider}")
        logger.info(f"NER provider: {config.ner_provider}")

        processor.run_continuous(args.chat_id)

    except KeyboardInterrupt:
        logger.info("Interrupted by user")

    finally:
        processor.cleanup()


def main():
    parser = argparse.ArgumentParser(
        description="ML Processor - Batch job for analyzing Portuguese chat messages"
    )
    subparsers = parser.add_subparsers(dest="command", required=True)

    # process command
    process_parser = subparsers.add_parser(
        "process",
        help="Process unanalyzed messages in batches",
    )
    process_parser.add_argument(
        "--chat-id",
        type=int,
        required=True,
        help="Target chat ID to process",
    )
    process_parser.add_argument(
        "--batch-size",
        type=int,
        default=None,
        help=f"Messages per batch (default: {_config.batch_size} from config)",
    )
    process_parser.add_argument(
        "--limit",
        type=int,
        default=None,
        help="Max messages to process (default: unlimited)",
    )
    process_parser.add_argument(
        "--from-date",
        type=str,
        default=None,
        help="Process messages from this date (YYYY-MM-DD)",
    )
    process_parser.add_argument(
        "--to-date",
        type=str,
        default=None,
        help="Process messages until this date (YYYY-MM-DD)",
    )

    # status command
    status_parser = subparsers.add_parser(
        "status",
        help="Show processing status for a chat",
    )
    status_parser.add_argument(
        "--chat-id",
        type=int,
        required=True,
        help="Target chat ID",
    )

    # continuous command
    continuous_parser = subparsers.add_parser(
        "continuous",
        help="Run continuous processing loop (daemon mode)",
    )
    continuous_parser.add_argument(
        "--chat-id",
        type=int,
        required=True,
        help="Target chat ID to process",
    )

    # generate-cards command
    generate_cards_parser = subparsers.add_parser(
        "generate-cards",
        help="Generate weekly user cards for a chat",
    )
    generate_cards_parser.add_argument(
        "--chat-id",
        type=int,
        required=True,
        help="Target chat ID to generate cards for",
    )
    generate_cards_parser.add_argument(
        "--week",
        type=str,
        default=None,
        help="Week start date (YYYY-MM-DD, should be Monday). Default: current week",
    )
    generate_cards_parser.add_argument(
        "--window-days",
        type=int,
        default=30,
        help="Rolling window size for stats calculation (default: 30)",
    )
    generate_cards_parser.add_argument(
        "--min-messages",
        type=int,
        default=10,
        help="Minimum messages required for card generation (default: 10)",
    )
    generate_cards_parser.add_argument(
        "--from-date",
        type=str,
        default=None,
        help="Stats window start date (YYYY-MM-DD). Overrides --window-days",
    )
    generate_cards_parser.add_argument(
        "--to-date",
        type=str,
        default=None,
        help="Stats window end date (YYYY-MM-DD). Overrides --window-days",
    )
    generate_cards_parser.add_argument(
        "--timezone",
        type=str,
        required=True,
        help="IANA timezone for week boundaries and chronotype (e.g., America/Sao_Paulo)",
    )

    # render-cards command
    render_cards_parser = subparsers.add_parser(
        "render-cards",
        help="Render card images for a chat (requires card-image-generator service)",
    )
    render_cards_parser.add_argument(
        "--chat-id",
        type=int,
        required=True,
        help="Target chat ID to render cards for",
    )
    render_cards_parser.add_argument(
        "--week",
        type=str,
        default=None,
        help="Week start date (YYYY-MM-DD). Default: latest week with cards",
    )
    render_cards_parser.add_argument(
        "--theme",
        type=str,
        default="gaming",
        help="Template theme name (default: gaming)",
    )
    render_cards_parser.add_argument(
        "--force",
        action="store_true",
        help="Force regenerate images even if they exist",
    )

    args = parser.parse_args()

    # Use module-level config (already loaded for New Relic init)
    config = _config

    # Setup logging
    setup_logging(config)

    # Validate API keys for non-local providers
    errors = config.validate_api_keys()
    if errors:
        for error in errors:
            logger.error(f"Configuration error: {error}")
        sys.exit(1)

    # Log startup info
    logger.info(f"ML Processor starting (device: {config.device})")
    logger.info(f"Database: {config.db_host}:{config.db_port}/{config.db_name}")
    logger.info(f"Qdrant: {config.qdrant_host}:{config.qdrant_port}")
    if config.new_relic_enabled():
        logger.info(f"New Relic APM: {config.new_relic_full_app_name}")

    try:
        if args.command == "process":
            result = run_process(args, config)
            sys.exit(0 if result >= 0 else 1)

        elif args.command == "status":
            run_status(args, config)
            sys.exit(0)

        elif args.command == "continuous":
            run_continuous(args, config)
            sys.exit(0)

        elif args.command == "generate-cards":
            run_generate_cards(args, config)
            sys.exit(0)

        elif args.command == "render-cards":
            result = run_render_cards(args, config)
            sys.exit(0 if result else 1)

    except Exception as e:
        logger.error(f"Fatal error: {e}", exc_info=True)
        sys.exit(1)


if __name__ == "__main__":
    main()
