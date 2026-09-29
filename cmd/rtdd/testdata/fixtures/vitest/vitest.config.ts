import { defineConfig } from "vitest/config";

// The thresholds are deliberately unreachable by one test file: the adapter must override them.
export default defineConfig({
  test: {
    include: ["src/**/*.test.ts"],
    coverage: { thresholds: { lines: 100, functions: 100, branches: 100, statements: 100 } },
  },
});
