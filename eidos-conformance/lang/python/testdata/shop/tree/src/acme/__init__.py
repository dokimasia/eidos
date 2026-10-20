"""The markers of the generator, which a module states as decorators."""

from collections.abc import Callable


def skip[T](*, plugin: str = "") -> Callable[[T], T]: ...
