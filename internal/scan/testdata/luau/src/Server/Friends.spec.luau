local Friends = require("./Friends")

local function check(cond, msg)
	if not cond then
		error(msg)
	end
end

return {
	{
		name = "a new list has no friends",
		fn = function()
			local f = Friends.new()
			check(not Friends.areFriends(f, 1, 2), "expected strangers")
		end,
	},
	{ name = "add links both", fn = function() local f = Friends.new(); Friends.add(f, 1, 2); check(Friends.areFriends(f, 1, 2), "linked") end },
}
