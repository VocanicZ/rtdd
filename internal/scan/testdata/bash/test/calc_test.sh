#!/usr/bin/env bash
source ./calc.sh

test_add() {
  [ "$(add 1 2)" = 3 ] && echo ok
}

test_double() {
  double 2 | grep -q 4
}

test_total() {
  true && total 1 2 3 >/dev/null
}
