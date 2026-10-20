"""The shop: its orders, its catalog and its services."""

from typing import Final

from shop.catalog import Color, Permission
from shop.models import Line, Money, Order, Point

__all__ = ["VERSION", "Color", "Line", "Money", "Order", "Permission", "Point"]

VERSION: Final = "1.0.0"
"""The version of the package."""
