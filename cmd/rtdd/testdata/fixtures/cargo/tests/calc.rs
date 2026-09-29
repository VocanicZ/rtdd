use rtdd_fixture_cargo::calc;

#[test]
fn adds() {
    assert_eq!(calc::add(1, 2), 3);
}
