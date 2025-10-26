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

    Sets up structured logging based on debug mode:
    - Debug mode: Pretty-printed colorful logs to stdout
    - Production: JSON-formatted logs to stdout for container logging
    """

    # Remove default logger
    logger.remove()

    if settings.debug:
        # Development: Pretty, colorful logs
        logger.add(
            sys.stdout,
            format="<green>{time:YYYY-MM-DD HH:mm:ss}</green> | "
                   "<level>{level: <8}</level> | "
                   "<cyan>{name}</cyan>:<cyan>{function}</cyan>:<cyan>{line}</cyan> | "
                   "<level>{message}</level>",
            level="DEBUG",
            colorize=True,
        )
    else:
        # Production: JSON logs for parsing
        logger.add(
            sys.stdout,
            format="{message}",
            level="INFO",
            serialize=True,  # JSON output
        )

    logger.info(f"Logging configured (debug={settings.debug})")
