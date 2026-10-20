//! The helpers of the shop.

extern crate std as standard;

use std::borrow::Cow;
use std::path::{Path, PathBuf};

use crate::models::Order as Purchase;

/// The most retries of a store.
pub const MAX_RETRIES: u32 = 3;

/// The greeting of the shop.
pub static GREETING: &str = "hello";

/// The absent code.
pub const EMPTY: Option<u8> = None;

/// Returns an option nested deeper than the rules follow.
#[allow(clippy::type_complexity)]
pub fn nested() -> Option<Option<Option<Option<Option<Option<Option<Option<Option<u8>>>>>>>>> {
    None
}

/// A cache of values.
pub struct Cache<K, V = String>
where
    K: std::hash::Hash + Eq,
{
    /// The entries of the cache.
    pub entries: std::collections::HashMap<K, V>,
}

/// Returns the name of a path.
pub fn name_of(path: &Path) -> Cow<'_, str> {
    path.to_string_lossy()
}

/// Returns a path below the root.
pub fn below(root: PathBuf, name: &str) -> PathBuf {
    root.join(name)
}

/// Returns the first order.
pub fn first(orders: &[Purchase]) -> Option<&Purchase> {
    orders.first()
}

/// Fails with a boxed error.
pub fn fail() -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
    Err("failed".into())
}

/// Sets the note of an order.
pub fn rename(order: &mut Purchase, note: String) {
    order.note = Some(note);
}

/// Returns the text of a value.
pub fn show<T: std::fmt::Display + Clone>(value: T) -> String {
    value.to_string()
}

/// Returns the default settings.
pub fn defaults() -> crate::models::Settings {
    crate::models::Settings::default()
}

/// The identifiers of orders.
pub type Ids = Vec<u64>;

/// The first of a chain of aliases longer than the rules follow.
pub type Deep0 = Deep1;
/// An alias of the chain.
pub type Deep1 = Deep2;
/// An alias of the chain.
pub type Deep2 = Deep3;
/// An alias of the chain.
pub type Deep3 = Deep4;
/// An alias of the chain.
pub type Deep4 = Deep5;
/// An alias of the chain.
pub type Deep5 = Deep6;
/// An alias of the chain.
pub type Deep6 = Deep7;
/// An alias of the chain.
pub type Deep7 = Deep8;
/// An alias of the chain.
pub type Deep8 = Deep9;
/// The last alias of the chain, a result.
pub type Deep9 = std::result::Result<u8, String>;

/// A module inside the file.
pub mod inner {
    use super::MAX_RETRIES;

    /// Returns the most retries.
    pub fn retries() -> u32 {
        MAX_RETRIES
    }
}
