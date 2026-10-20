//! The errors of the shop.

use std::fmt;

use thiserror::Error;

/// The error of a store.
#[derive(Debug, Error)]
pub enum StoreError {
    /// The order does not exist.
    #[error("the order does not exist")]
    NotFound,
    /// The store failed to read.
    #[error("the store failed to read")]
    Io(#[from] std::io::Error),
}

/// The error of a parse of an order.
#[derive(Debug)]
pub struct ParseError {
    /// The line of the error.
    pub line: u32,
    /// The error of the number.
    pub source: std::num::ParseIntError,
}

impl fmt::Display for ParseError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "line {}", self.line)
    }
}

impl std::error::Error for ParseError {}

/// The error of a load, whose cause thiserror marks.
#[derive(Debug, Error)]
#[error("the order failed to load")]
pub struct LoadError {
    /// The identifier of the order.
    pub id: u64,
    /// The cause.
    #[source]
    pub cause: ParseError,
}

/// The result of the crate.
pub type Result<T> = std::result::Result<T, StoreError>;

/// An outcome whose failure borrows its text.
pub type Outcome<'a, T = u8> = std::result::Result<T, &'a str>;
