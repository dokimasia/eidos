from typing import Protocol


class Reader(Protocol):
    def read(self) -> str: ...
