import { add, sum } from './calc';
import { clamp } from './util';

describe('calc', () => {
  it('adds', () => {
    expect(add(1, 2)).toBe(3);
  });

  test("sums", async () => {
    expect(await sum([1, 2, 3])).toBe(6);
  });
});

it(`clamps`, () => {
  expect(clamp(9, 0, 5)).toBe(5);
});
