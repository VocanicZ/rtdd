local M = {}

function M.new()
	return { links = {} }
end

function M.add(f, a, b)
	f.links[a .. ":" .. b] = true
end

function M.areFriends(f, a, b)
	return f.links[a .. ":" .. b] == true
end

return M
