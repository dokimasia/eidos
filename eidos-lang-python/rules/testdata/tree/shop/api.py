"""The API of the shop."""

import shop.models
from shop import Order
from shop.models import *


def fetch(order_id: int) -> Order:
    return Order(order_id, UserId(1), [])


def latest() -> shop.models.Order:
    return fetch(1)


def favourite() -> Status:
    return Status.NEW
