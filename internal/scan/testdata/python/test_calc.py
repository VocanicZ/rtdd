from calc import Calc, add, total


def test_add():
    assert add(1, 2) == 3


class TestCalc:
    def test_plus(self):
        c = Calc()
        assert c.plus(2) == 2

    def test_total(self):
        assert total([1, 2]) == 3
