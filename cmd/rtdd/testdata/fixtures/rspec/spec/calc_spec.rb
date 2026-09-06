require "calc"

RSpec.describe Calc do
  it "adds" do
    expect(Calc.add(1, 2)).to eq(3)
  end
end
