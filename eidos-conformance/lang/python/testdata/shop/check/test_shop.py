"""The behaviour of the declarations of the mirror of the shop."""

import unittest
from datetime import datetime

from shop.catalog import Color, Permission
from shop.contracts import Customer
from shop.forms import Coord, Shade, Span
from shop.generics import Box, Sku
from shop.models import MAX_LINES, Line, Money, Order, Point


class ShopTest(unittest.TestCase):
    def test_order_defaults(self) -> None:
        order = Order(number=1, placed=datetime(2026, 10, 20))
        self.assertEqual(order.lines, [])
        self.assertEqual(order.channel, "web")
        self.assertIsNone(order.note)

    def test_line_defaults(self) -> None:
        line = Line("A-1")
        self.assertEqual(line.quantity, 1)
        self.assertIs(line.kind, Line.Kind.GOOD)

    def test_money_is_frozen(self) -> None:
        money = Money(150)
        self.assertEqual(money.currency, "EUR")
        with self.assertRaises(AttributeError):
            setattr(money, "currency", "USD")

    def test_point_is_a_tuple(self) -> None:
        self.assertEqual(Point(3), (3, 0))

    def test_color_values(self) -> None:
        self.assertEqual(Color.RED.value, "red")
        self.assertEqual(Color.fallback, "red")
        self.assertEqual(len(Color), 3)

    def test_permissions_combine(self) -> None:
        self.assertEqual(Permission.READ | Permission.WRITE, 3)

    def test_customer_is_a_dict(self) -> None:
        customer: Customer = {"name": "Ada", "address": {"street": "Main", "city": "Delft"}}
        self.assertEqual(customer["name"], "Ada")

    def test_new_type(self) -> None:
        self.assertEqual(Sku("A-1"), "A-1")

    def test_generic_class(self) -> None:
        self.assertIsInstance(Box[Line](), Box)

    def test_constant(self) -> None:
        self.assertEqual(MAX_LINES, 100)

    def test_functional_forms(self) -> None:
        self.assertEqual(Span(1), (1, 0))
        self.assertEqual(Coord(1, 2).y, 2)
        self.assertEqual(Shade.DARK.value, 2)


if __name__ == "__main__":
    unittest.main()
