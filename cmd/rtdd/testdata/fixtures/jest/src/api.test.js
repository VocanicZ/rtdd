const { handle } = require("./api");
const { fixtureKey } = require("../__tests__/helpers");

test("handles", () => {
  expect(handle(fixtureKey)).toBe("v:a");
});
