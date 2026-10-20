"""The stock of the warehouse."""

from dataclasses import dataclass
from typing import Annotated, Literal, Self, final, override

import acme

from shop.models import *

type Grade = Literal["new", "used", "refurbished"]


class Slot:
    """A place in the warehouse that holds one product."""

    def __new__(cls, code: str) -> Self: ...

    def describe(self) -> str: ...


@final
class Bin(Slot):
    """A slot for small products."""

    weights: tuple[int, ...]
    limit: Annotated[int, "grams"] = 1000
    price: Money | None = None

    @override
    def describe(self) -> str: ...


#+acme:skip plugin=clientgen
@dataclass
class Audit:
    """A count of the stock that the client leaves out."""

    counted: int


@acme.skip(plugin="modelgen")
@dataclass
class Recount:
    """A second count of a bin that the model leaves out."""

    grade: Grade
