from src.calc import add, total


def test_add():
    assert add(1, 2) == 3


def test_total():
    assert total([1, 2]) == 3
