//! The models of the shop.

use std::collections::{BTreeMap, HashMap, HashSet};
use std::sync::Arc;
use std::time::{Duration, SystemTime};

/// The state of an order.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub enum Status {
    /// A new order.
    #[default]
    New,
    /// A paid order.
    Paid = 2,
    /// A shipped order.
    Shipped,
}

/// A color, which has no default.
#[derive(Debug, Clone, Copy)]
pub enum Color {
    /// Red.
    Red,
    /// Green.
    Green,
}

/// The identifier of an order.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub struct Id(pub u64);

impl Default for Id {
    fn default() -> Self {
        Id(1)
    }
}

/// One line of an order.
#[derive(Debug, Clone, PartialEq)]
pub struct Item {
    /// The stock keeping unit.
    pub sku: String,
    /// The number of units.
    pub quantity: u32,
    /// The price of one unit.
    pub price: f64,
}

impl Item {
    /// Returns a line of one unit of the stock keeping unit, where it is not
    /// empty.
    pub fn checked(sku: String) -> Option<Item> {
        if sku.is_empty() {
            return None;
        }
        Some(Item { sku, quantity: 1, price: 0.0 })
    }
}

/// The state of an order, under another name.
pub type State = Status;

/// An enum of one value.
pub enum Single {
    /// The only value.
    Only,
}

/// An enum without values.
pub enum Never {}

/// A moment, of which only the variant without fields has a value.
pub enum Moment {
    /// No moment.
    Unset,
    /// A moment of the clock.
    At(std::time::Instant),
}

/// A moment of the clock that is always set.
pub enum Clock {
    /// The start.
    Started(std::time::Instant),
    /// The end.
    Stopped(std::time::Instant),
}

/// An event.
pub enum Event {
    /// A move to a place.
    Moved {
        /// The column.
        x: i32,
        /// The row.
        y: i32,
    },
    /// The end of the moves.
    Stopped,
}

/// An order.
#[derive(Debug, Clone, PartialEq)]
pub struct Order {
    /// The identifier.
    pub id: Id,
    /// The state.
    pub status: Status,
    /// The lines.
    pub items: Vec<Item>,
    /// The note of the customer.
    pub note: Option<String>,
    /// The tags.
    pub tags: HashSet<String>,
    /// The totals by currency.
    pub totals: HashMap<String, f64>,
    /// The ranks by level.
    pub ranks: BTreeMap<u8, i64>,
    /// The time of the order.
    pub placed: SystemTime,
    /// The time after which the order expires.
    pub timeout: Duration,
    /// The code of the order.
    pub code: [u8; 4],
    /// A pair of values.
    pub pair: (i32, bool),
    /// The label.
    pub label: &'static str,
    /// The initial of the customer.
    pub initial: char,
    /// The name of the shared owner.
    pub shared: Arc<str>,
}

impl Order {
    /// Returns an order of the identifier without lines.
    pub fn new(id: Id, status: Status) -> Self {
        Order {
            id,
            status,
            items: Vec::new(),
            note: None,
            tags: HashSet::new(),
            totals: HashMap::new(),
            ranks: BTreeMap::new(),
            placed: SystemTime::UNIX_EPOCH,
            timeout: Duration::ZERO,
            code: [0; 4],
            pair: (0, false),
            label: "",
            initial: 'a',
            shared: Arc::from(""),
        }
    }

    /// Parses an order from the text of its identifier.
    pub fn parse(text: &str) -> crate::errors::Result<Order> {
        let id = text.parse().map_err(|_| crate::errors::StoreError::NotFound)?;
        Ok(Order::new(Id(id), Status::New))
    }

    /// Returns the sum of the prices of the lines.
    pub fn total(&self) -> f64 {
        self.items.iter().map(|i| i.price * f64::from(i.quantity)).sum()
    }

    /// Returns a blank order, which the module alone builds.
    fn blank() -> Self {
        Order::new(Id(0), Status::New)
    }
}

/// A shape.
#[derive(Debug, Clone, PartialEq, Default)]
pub enum Shape {
    /// No shape.
    #[default]
    Empty,
    /// A circle of a radius.
    Circle(f64),
    /// A square of a side.
    Square {
        /// The length of a side.
        side: f64,
    },
}

/// A page of values.
#[derive(Debug, Clone, PartialEq)]
pub struct Page<T> {
    /// The values of the page.
    pub items: Vec<T>,
    /// The number of the next page.
    pub next: Option<u32>,
}

/// The settings of a store, which a caller cannot build field by field.
#[derive(Debug, Default)]
pub struct Settings {
    /// The number of retries.
    pub retries: u8,
    /// The level of the log, which the crate alone sets.
    pub(crate) level: u8,
    secret: String,
}

/// A counter behind a lock, which does not compare.
pub struct Guarded {
    /// The count.
    pub count: std::sync::Mutex<u32>,
}

/// A tree of values.
#[derive(Debug, Clone, PartialEq)]
pub enum Tree<T> {
    /// A leaf with its value.
    Leaf(T),
    /// A node with two subtrees.
    Node(Box<Tree<T>>, Box<Tree<T>>),
}

/// A unit of nothing.
#[derive(Debug, Clone, Copy, PartialEq, Default)]
pub struct Nothing;

/// A chain of links, of which no value is finite.
pub struct Link {
    /// The next link.
    pub next: Box<Link>,
}
