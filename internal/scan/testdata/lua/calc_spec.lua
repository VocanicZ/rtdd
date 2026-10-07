local calc = require("calc")

describe("calc", function()
  it("totals a list", function()
    assert.equals(6, calc.total({ 1, 2, 3 }))
  end)
end)
