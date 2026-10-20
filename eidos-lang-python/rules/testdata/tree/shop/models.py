"""The models of the shop."""

import dataclasses
from collections.abc import AsyncIterator, Iterator, Mapping, Sequence
from dataclasses import KW_ONLY, InitVar, dataclass, field
from datetime import datetime, timedelta
from enum import Enum, Flag, IntFlag, StrEnum, auto
from typing import Generator, Literal, NamedTuple, NewType, NotRequired, Protocol, TypedDict, overload

UserId = NewType("UserId", int)

type Tags = list[str]

type Pair[K, V] = tuple[K, V]

Mode = Literal["fast", "slow"]


class Status(Enum):
    """The status of an order."""

    NEW = 1
    PAID = 2
    SHIPPED = auto()
    LEGACY = 1


class Color(StrEnum):
    RED = auto()
    GREEN = auto()


class Perm(IntFlag):
    READ = auto()
    WRITE = auto()
    EXEC = auto()


class Shape(Flag):
    NONE = 0
    ROUND = auto()
    SQUARE = auto()


class Level(Enum):
    LOW = "low"
    HIGH = "high"


class Same(Enum):
    A = 1
    B = 1


class Lowered(StrEnum):
    """A base of enums whose values are the names of their members in lower case."""


class Mood(Lowered):
    HAPPY = auto()


class Ordinal(Enum):
    @staticmethod
    def _generate_next_value_(name: str, start: int, count: int, last_values: list[int]) -> int:
        return count * 10

    FIRST = auto()


@dataclass
class Item:
    """One line of an order."""

    sku: str
    quantity: int = 1
    tags: list[str] = field(default_factory=list)
    cached: str = field(default="", init=False)


@dataclass(frozen=True)
class Money:
    amount: int
    currency: str = "EUR"


@dataclass(kw_only=True)
class Address:
    street: str
    city: str


@dataclasses.dataclass
class Order:
    """An order of the shop."""

    id: int
    customer: UserId
    items: list[Item]
    _: KW_ONLY
    status: Status = Status.NEW
    note: str | None = None


@dataclass
class Rush(Order):
    deadline: datetime = field(default_factory=datetime.now)


class Point(NamedTuple):
    x: int
    y: int = 0


class Payload(TypedDict, total=False):
    name: str
    count: NotRequired[int]


class Required(TypedDict):
    key: str


class BadDict(TypedDict):
    seen: set[int]


class Credentials(TypedDict):
    user: str
    token: str


class Nothing(TypedDict, total=False):
    seen: set[int]


@dataclass
class Tricky:
    a: int = field(default=0, metadata=dict(x=1, kw_only=True, y=2))
    b: str = field(default="p, kw_only=True, q")
    c: str = field(default="\", kw_only=True, \"")


@dataclass
class Defaults:
    seen: set[int] = field(default_factory=set)
    label: str = "x"


@dataclass
class Bag:
    items: set[int]


class Pinned:
    def __init__(self, code: int, /) -> None:
        self.code = code


class Named(Protocol):
    @property
    def name(self) -> str: ...


class Account:
    def __init__(self, owner: str, *, limit: int = 100) -> None:
        self.owner = owner
        self._limit = limit

    @property
    def limit(self) -> int:
        return self._limit

    @limit.setter
    def limit(self, value: int) -> None:
        self._limit = value

    @limit.deleter
    def limit(self) -> None:
        del self._limit

    def __eq__(self, other: object) -> bool:
        return isinstance(other, Account) and other.owner == self.owner


class Token:
    def __new__(cls, text: str) -> "Token":
        return super().__new__(cls)


class Parsed:
    @overload
    def __init__(self, value: int) -> None: ...

    @overload
    def __init__(self, value: str) -> None: ...

    def __init__(self, value: int | str) -> None:
        self.value = value


class Plain:
    label: str = "plain"


class Outer:
    class Inner:
        size: int


class OutOfStock(ValueError):
    """An item is not in stock."""

    def __str__(self) -> str:
        return "out of stock"


class Box[T: int = int]:
    value: T


@dataclass
class Linked:
    next: "Linked"


type Ping = Pong

type Pong = Ping


class Catalog:
    sizes: tuple[int, str]
    weights: Sequence[float]
    prices: Mapping[str, Money]
    counts: dict[str, int]
    seen: set[int]
    flags: bytes
    when: datetime
    wait: timedelta
    mode: Mode
    kind: Literal["a", "b"]
    single: Literal["x"]
    favourite: Literal[Status.NEW, Status.PAID]
    tags: Tags
    owner: UserId
    either: int | str
    maybe: bool | None
    anything: object
    pair: Pair[int, str]
    lists: list[int] | list[str]


def stream() -> Iterator[int]:
    yield 1


def produce() -> Generator[str, None, None]:
    yield "a"


async def watch() -> AsyncIterator[Order]:
    yield Order(1, UserId(1), [])


def total(order: Order, /, factor: float = 1.0) -> int:
    return len(order.items)


LIMIT = 10


class Vault:
    def __init__(self) -> None:
        self.__secret = 1


class Safe(Vault):
    pass


class SoldOut(OutOfStock):
    pass


class Silent(Exception):
    pass


class Gauge:
    @property
    def level(self):
        return 1


@dataclass(init=False)
class Manual:
    x: int

    def __init__(self, y: str) -> None:
        self.x = len(y)


@dataclass
class Mixed:
    d: int = field(repr=False)
    a: int = 0
    b: int = field(kw_only=True, default=0)


@dataclass
class Base1:
    x: int
    y: int = 0


@dataclass
class Child1(Base1):
    x: int = 5


class Impl(Named):
    @property
    def name(self) -> str:
        return "impl"


@dataclass(frozen=True)
class Holder:
    items: list[int]


@dataclass(unsafe_hash=True)
class Keyed:
    x: int


@dataclass(eq=False)
class Ident:
    x: list[int]


class NoHash:
    __hash__ = None


class Both:
    def __eq__(self, other: object) -> bool:
        return True

    def __hash__(self) -> int:
        return 1


class Row(NamedTuple):
    cells: list[int]


class Derived(Account):
    pass


class Rehashed(Account):
    def __hash__(self) -> int:
        return 1


class Numbers(list[int]):
    pass


@dataclass(frozen=True)
class Grown(Holder):
    extra: int = 0


@dataclass(eq=False)
class Plainly(Account):
    x: int = 0


def account() -> Account:
    return Account("a")


def maybe_order() -> Order | None:
    return None


def tagged() -> Tags:
    return []


def vague() -> "Missing":
    return None


class Chained(Exception):
    __cause__: Exception | None


@dataclass(frozen=True)
class Seeded:
    key: int
    seed: InitVar[list[int]]
