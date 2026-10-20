"""The contracts that the services of the shop rely on."""

from abc import ABC, abstractmethod
from typing import (
    NotRequired,
    Protocol,
    ReadOnly,
    Required,
    TypedDict,
    runtime_checkable,
)

from shop.models import Money


@runtime_checkable
class Priced(Protocol):
    """Anything with a price."""

    @property
    def price(self) -> Money: ...

    def discounted(self, percent: int) -> Money: ...


class Address(TypedDict, total=False):
    """The address of a customer."""

    street: Required[str]
    city: Required[str]
    postcode: str
    country: ReadOnly[str]


class Customer(TypedDict):
    """A customer of the shop."""

    name: str
    email: NotRequired[str]
    address: Address


class Gateway(ABC):
    """A provider of payments."""

    @abstractmethod
    def charge(self, amount: Money) -> bool: ...

    def refund(self, amount: Money) -> bool:
        """Refunds an amount, which a gateway refuses unless it states otherwise."""
        ...
