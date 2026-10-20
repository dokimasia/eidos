"""The orders of the shop and their lines."""

from dataclasses import InitVar, dataclass, field
from datetime import datetime
from enum import StrEnum
from typing import ClassVar, Final, NamedTuple

MAX_LINES: Final = 100
"""The most lines that one order has."""

default_currency: str = "EUR"


@dataclass(frozen=True)
class Money:
    """An amount of money in one currency."""

    cents: int
    currency: str = "EUR"


class Point(NamedTuple):
    """A place in the warehouse."""

    aisle: int
    shelf: int = 0  # the floor of the aisle


@dataclass
class Line:
    """One product of an order."""

    class Kind(StrEnum):
        """What a line sells."""

        GOOD = "good"
        SERVICE = "service"

    sku: str
    kind: Kind = Kind.GOOD
    quantity: int = 1
    """The number of items, at least one."""
    price: Money | None = None


@dataclass(kw_only=True)
class Order:
    """An order of a customer."""

    CAPACITY: ClassVar[int] = 100
    instances: ClassVar[int] = 0
    number: int
    placed: datetime
    lines: list[Line] = field(default_factory=list[Line])
    note: str | None = None
    channel: Final[str] = "web"
    coupon: InitVar[str | None] = None

    def __post_init__(self, coupon: str | None) -> None: ...

    def total(self) -> Money:
        """Returns the sum of the prices of the lines."""
        ...
