require "api"

RSpec.describe Api do
  it "handles" do
    expect(Api.handle("a")).to eq("v:a")
  end
end
