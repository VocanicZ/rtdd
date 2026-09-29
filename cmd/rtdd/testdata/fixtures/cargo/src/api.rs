use crate::store;

pub fn handle(k: &str) -> String {
    store::get(k)
}
