local M = {}

local function add(a, b)
  return a + b
end

function M.load(path)
  local f = io.open(path)
  return f
end

function M:push(n)
  self.total = add(self.total or 0, n)
  return self
end

function M.total(list)
  local acc = M.new()
  for _, n in ipairs(list) do
    acc:push(n)
  end
  return acc.total
end

function M.new()
  return setmetatable({}, { __index = M })
end

return M
