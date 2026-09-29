require "store"

module Api
  def self.handle(key)
    Store.get(key)
  end
end
