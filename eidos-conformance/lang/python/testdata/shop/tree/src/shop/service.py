# pyright: strict
"""The services of the shop."""

import json
lazy import logging
from decimal import Decimal
from typing import Self, Unpack, overload

from shop.contracts import Customer, Gateway
from shop.models import Line, Order

registry: dict[int, Order] = {}


class Cart:
    """The lines that a customer collects before an order."""

    __slots__ = ("_customer", "_lines")

    def __init__(self, customer: str) -> None: ...

    @property
    def customer(self) -> str:
        """The name of the customer."""
        ...

    @customer.setter
    def customer(self, value: str) -> None: ...

    @customer.deleter
    def customer(self) -> None: ...

    @classmethod
    def empty(cls) -> Self: ...

    @staticmethod
    def tax(amount: Decimal) -> Decimal: ...


@overload
def find(number: int) -> Order: ...


@overload
def find(number: str) -> Order | None: ...


def find(number: int | str) -> Order | None:
    """Returns the order of a number, or of the text of a number."""
    ...


def quote(line: Line, /, *, currency: str = "EUR", rounding: int = 2) -> str: ...


def register(**fields: Unpack[Customer]) -> None: ...


def decoder() -> json.JSONDecoder: ...


def logger() -> logging.Logger: ...


async def settle(order: Order, gateway: Gateway) -> bool: ...
