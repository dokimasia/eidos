//! The stores of orders.

use std::io;

use crate::errors::{Result, StoreError};
use crate::models::*;

/// A store of orders.
pub trait Store {
    /// Returns the order of the identifier.
    fn get(&self, id: u64) -> Result<Order>;
    /// Lists the orders.
    fn list(&self) -> Box<dyn Iterator<Item = Order> + '_>;
    /// Returns the identifiers of the orders.
    fn ids(&self) -> impl Iterator<Item = u64>;
    /// Saves an order.
    fn save(&mut self, order: &Order) -> std::result::Result<(), StoreError>;
    /// Flushes the store.
    fn flush(&self) -> io::Result<()>;
    /// Writes the store into out.
    fn write(&self, out: &mut String) -> std::fmt::Result;
    /// Returns the number of the orders.
    fn len(&self) -> usize;
}

/// A store that has a name.
pub trait Named: Store {
    /// The key of the name.
    type Key;

    /// Returns the name of the store.
    fn name(&self) -> String;
}

/// A store in memory.
#[derive(Debug, Default)]
pub struct Memory {
    /// The orders of the store.
    pub orders: Vec<Order>,
}

impl Memory {
    /// The most orders of a store.
    pub const LIMIT: usize = 10;

    /// Returns an empty store.
    pub fn new() -> Self {
        Memory { orders: Vec::new() }
    }

    /// Returns the number of the orders.
    pub fn len(&self) -> usize {
        self.orders.len()
    }
}

impl Store for Memory {
    fn get(&self, id: u64) -> Result<Order> {
        self.orders.iter().find(|o| o.id.0 == id).cloned().ok_or(StoreError::NotFound)
    }

    fn list(&self) -> Box<dyn Iterator<Item = Order> + '_> {
        Box::new(self.orders.iter().cloned())
    }

    fn ids(&self) -> impl Iterator<Item = u64> {
        self.orders.iter().map(|o| o.id.0)
    }

    fn save(&mut self, order: &Order) -> std::result::Result<(), StoreError> {
        self.orders.push(order.clone());
        Ok(())
    }

    fn flush(&self) -> io::Result<()> {
        Ok(())
    }

    fn write(&self, out: &mut String) -> std::fmt::Result {
        out.push_str("memory");
        Ok(())
    }

    fn len(&self) -> usize {
        self.orders.len()
    }
}

/// A source of labels.
pub trait Labelled {
    /// Returns the label.
    fn label(&self) -> String;
}

/// Another source of labels.
pub trait Tagged {
    /// Returns the label.
    fn label(&self) -> String;
}

/// A shelf with two labels.
pub struct Shelf;

impl Labelled for Shelf {
    fn label(&self) -> String {
        String::from("labelled")
    }
}

impl Tagged for Shelf {
    fn label(&self) -> String {
        String::from("tagged")
    }
}

impl PartialEq for Shelf {
    fn eq(&self, _: &Self) -> bool {
        true
    }
}

/// Returns the open orders of a store.
pub fn open(store: &impl Store, status: Status) -> Vec<Order> {
    store.list().filter(|o| o.status == status).collect()
}

/// Returns the label of a source of labels.
pub fn label_of(source: &dyn Labelled) -> String {
    source.label()
}

/// Finds an order.
pub async fn find(store: &impl Store, id: u64) -> Option<Order> {
    store.get(id).ok()
}
