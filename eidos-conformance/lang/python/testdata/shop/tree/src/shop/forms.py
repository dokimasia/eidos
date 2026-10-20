"""The forms that the mirror of the shop writes in another spelling."""

from __future__ import annotations

import sys
from collections import namedtuple
from enum import Enum
from typing import TYPE_CHECKING, NamedTuple, Optional, TypedDict, Union

from .models import Money

if TYPE_CHECKING:
    from .catalog import Color

Coord = NamedTuple("Coord", [("x", int), ("y", int)])

Span = namedtuple("Span", "left right", defaults=[0])

Movie = TypedDict("Movie", {"title": str, "year-of": int})

Shade = Enum("Shade", "LIGHT DARK")


def total(amount, currency):
    # type: (int, str) -> Money
    ...


def tint(color: Optional[Color], fallback: Union[int, str]) -> "Money": ...


if sys.platform == "win32":
    SEP: str = "\\"
else:
    SEP: str = "/"
