"""
Logging configuration using loguru.

Provides structured logging with JSON output in production
and pretty-printed output in development.

Pattern from wt_api_v2 (validated for production use).
"""

import sys

from loguru import logger

from app.config.settings import settings


def setup_logging() -> None:
    """Configure loguru logger.

    Sets up structured logging based on LOG_LEVEL and LOG_FORMAT:
    - text format: Pretty-printed colorful logs to stdout
    - json format: JSON-formatted logs to stdout for container logging
    """

    # Remove default logger
    logger.remove()

    # Normalize log level to uppercase
    level = settings.log_level.upper()

    if settings.log_format.lower() == "text":
        # Text format: Pretty, colorful logs
        logger.add(
            sys.stdout,
            format="<green>{time:YYYY-MM-DD HH:mm:ss}</green> | "
                   "<level>{level: <8}</level> | "
                   "<cyan>{name}</cyan>:<cyan>{function}</cyan>:<cyan>{line}</cyan> | "
                   "<level>{message}</level>",
            level=level,
            colorize=True,
        )
    else:
        # JSON format: JSON logs for parsing
        logger.add(
            sys.stdout,
            format="{message}",
            level=level,
            serialize=True,  # JSON output
        )

    logger.info(f"Logging configured (level={level}, format={settings.log_format})")
