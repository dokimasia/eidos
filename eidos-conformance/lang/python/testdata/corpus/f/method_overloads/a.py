from typing import overload


class Box:
    @overload
    def Fill(self, x: int) -> None: ...

    @overload
    def Fill(self) -> None: ...

    def Fill(self, x: int | None = None) -> None:
        pass
