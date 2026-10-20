from collections.abc import Callable

from f.composite_refs.dep.d import Target


class Holder:
    f0: Target | None
    f1: dict[str, Target]
    f2: Callable[[Target], Exception]
