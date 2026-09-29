import { expect, test } from "vitest";
import { handle } from "./api";
import { fixtureKey } from "../tests/helpers";

test("handles", () => {
  expect(handle(fixtureKey)).toBe("v:a");
});
