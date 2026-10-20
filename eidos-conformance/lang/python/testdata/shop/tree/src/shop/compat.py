"""The parts of the shop that differ between versions of Python."""

import sys
from collections.abc import AsyncIterator
from typing import Final

from shop.models import Order

PLATFORM: Final = sys.platform
"""The platform that the shop runs on."""

if sys.version_info >= (3, 14):

    def annotations_of(obj: object) -> dict[str, object]:
        """Returns the annotations of an object, which Python 3.14 evaluates on demand."""
        ...

else:

    def annotations_of(obj: object) -> dict[str, object]:
        """Returns the annotations of an object."""
        ...


async def stream(customer: str) -> AsyncIterator[Order]:
    """Yields the orders of a customer as they arrive."""
    yield Order(number=0, placed=__import__("datetime").datetime.now())


class Ledger:
    """The orders that a session has seen."""

    def __init__(self) -> None:
        self.seen: list[Order] = []
        self.count = 0
