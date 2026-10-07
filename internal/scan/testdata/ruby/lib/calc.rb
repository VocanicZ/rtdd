module Arith
  def self.add(a, b)
    a + b
  end

  def self.twice(n)
    add n, n
  end
end

class Calc
  attr_reader :total

  def initialize
    @total = 0
  end

  def push(n)
    @total = Arith.add(@total, n)
    log n
  end

  def log(n)
    puts "pushed #{n}"
  end
end
