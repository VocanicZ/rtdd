// The threshold is deliberately unreachable by one test file: the adapter must override it.
module.exports = { testEnvironment: "node", testMatch: ["**/*.test.js"], coverageThreshold: { global: { lines: 100, functions: 100 } } };
