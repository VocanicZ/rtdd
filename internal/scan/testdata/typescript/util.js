'use strict';

function clamp(n, lo, hi) {
  return Math.min(Math.max(n, lo), hi);
}

const double = n => n * 2;

module.exports = { clamp, double };
