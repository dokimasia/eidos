use std::collections::HashMap;

pub mod dep;

pub struct Holder {
    f0: Option<dep::Target>,
    f1: HashMap<String, dep::Target>,
    f2: fn(dep::Target) -> Result<(), String>,
}
