"""The generic containers and the aliases of the shop."""

from __future__ import annotations

from collections.abc import Callable, Iterator
from typing import Generic, NewType, TypeAlias, TypeVar

from shop.models import Line

T_co = TypeVar("T_co", bound=Line, covariant=True, default=Line)

Sku = NewType("Sku", str)
"""The stock keeping unit of a product."""

type Lines = list[Line]

Pair: TypeAlias = tuple[T_co, T_co]


class Box[T: Line = Line]:
    """A line that waits for its order."""

    item: T

    def open(self) -> T: ...


class Feed(Generic[T_co]):
    """A stream of lines that a reader takes in order."""

    def __iter__(self) -> Iterator[T_co]: ...


def first[T](items: Iterator[T]) -> T: ...


def apply[**P, R](fn: Callable[P, R], *args: P.args, **kwargs: P.kwargs) -> R: ...


class Row[*Ts]:
    """A row of values of the types Ts."""

    def values(self) -> tuple[*Ts]: ...
