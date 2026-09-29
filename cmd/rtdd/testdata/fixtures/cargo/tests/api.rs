use rtdd_fixture_cargo::api;

#[test]
fn handles() {
    assert_eq!(api::handle("a"), "v:a");
}
