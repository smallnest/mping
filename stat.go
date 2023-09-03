package main

type Result struct {
	ts       int64
	seq      uint16
	target   string
	ttl      int64
	received bool
}

type TargetResult struct {
	ttl      int64
	sent     int
	received int
}
