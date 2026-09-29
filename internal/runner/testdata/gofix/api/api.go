package api

import "example.com/gofix/store"

func Handle(k string) string { return store.Get(k) }
