"""The classes that a dataclass_transform of PEP 681 makes dataclasses."""

from typing import Any, dataclass_transform


def specified(*, default: Any = ..., init: bool = True, alias: str | None = None) -> Any: ...


@dataclass_transform(kw_only_default=True, field_specifiers=(specified,))
def model(cls: type) -> type:
    return cls


@model
class Order:
    id: int
    note: str = specified(default="", alias="memo")
    total: int = specified(init=False)


@model(kw_only=False)
class Line:
    sku: str
    count: int = 1


@dataclass_transform(frozen_default=True)
class ModelBase:
    pass


class Item(ModelBase):
    sku: str
    tags: list[str]
    count: int = 0


@dataclass_transform()
class ModelMeta(type):
    pass


class Record(metaclass=ModelMeta):
    key: str


class Entry(Record, kw_only=True):
    value: int
