"""A tiny calculator."""


def add(a, b):
    return a + b


def total(items):
    result = 0
    for item in items:
        result = add(result, item)
    return result


class Calc:
    def __init__(self):
        self.memory = 0

    def plus(self, x):
        self.memory = add(self.memory, x)
        return self.memory

    def run(
        self,
        values,
    ):
        def step(v):
            return self.plus(v)

        return [step(v) for v in values]
