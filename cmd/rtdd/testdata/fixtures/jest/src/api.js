const { get } = require("./store");

function handle(k) {
  return get(k);
}

module.exports = { handle };
