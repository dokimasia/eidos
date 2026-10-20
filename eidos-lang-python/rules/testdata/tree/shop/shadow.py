"""A module whose names hide and rename the names that the rules read."""

from enum import Enum
from enum import auto as make


def auto() -> int:
    return 1


class Shadowed(Enum):
    A = auto()


class Made(Enum):
    A = make()
