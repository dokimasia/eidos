//! A client of the shop, in a crate of its own.

use shop::models::Settings;

/// Returns the settings of the client.
pub fn settings() -> Settings {
    Settings::default()
}
