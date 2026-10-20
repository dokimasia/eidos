"""Declarations that CPython cannot import: cycles of bases and constructs that the runtime refuses."""

import dataclasses
from enum import Enum, auto
from typing import ClassVar, NewType, TypedDict

Bare = NewType("Bare")


class Loose(TypedDict, total=False):
    version: ClassVar[int]
    seen: set[int]
    name: str


class Left(Right, Enum):
    A = auto()


class Right(Left, Enum):
    pass


class Up(Down):
    pass


class Down(Up):
    pass


@dataclasses.dataclass
class First(Second):
    pass


@dataclasses.dataclass
class Second(First):
    pass


class Unbound(Enum):
    A = never.auto()


class Nested(Enum):
    A = Enum.auto()
