require_relative "../lib/calc"

describe Calc do
  it "pushes" do
    c = Calc.new
    c.push(2)
    expect(c.total).to eq(2)
  end

  it 'doubles' do
    expect(Arith.twice(3)).to eq(6)
  end
end
