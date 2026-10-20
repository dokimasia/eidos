"""The colors and the permissions of the catalog."""

from enum import IntFlag, StrEnum, auto, nonmember
from typing import cast


class Color(StrEnum):
    """The color of a product."""

    RED = "red"
    GREEN = "green"
    BLUE = "blue"
    label: str
    fallback = nonmember(cast("str", "red"))

    def hex(self) -> str: ...


class Permission(IntFlag):
    """What a user may do with an order."""

    READ = auto()
    WRITE = auto()
    DELETE = auto()
