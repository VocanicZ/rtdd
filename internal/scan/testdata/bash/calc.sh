#!/usr/bin/env bash

add() {
  echo $(( $1 + $2 ))
}

function double {
  add "$1" "$1"
}

function total() {
  local sum=0
  for n in "$@"; do
    sum=$(add "$sum" "$n")
  done
  echo "$sum"
}
