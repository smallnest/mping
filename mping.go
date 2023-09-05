package main

import (
	"flag"
	"log"
	"time"

	"github.com/spf13/pflag"
)

var (
	count      = pflag.IntP("count", "c", 0, "count, 0 means non-setting")
	tos        = pflag.IntP("tos", "z", 0, "tos, 0 means non-setting")
	packetSize = pflag.IntP("size", "s", 64, "packet size")
	timeout    = pflag.DurationP("timeout", "t", time.Second, "timeout")
	rate       = pflag.IntP("rate", "r", 100, "rate, 100 means 100 packets per second for each target")
	delay      = pflag.IntP("delay", "d", 3, "delay seconds")
)

var (
	msgPrefix   = []byte("smallnest")
	targetAddrs []string
	stat        *buckets
)

func main() {
	pflag.Parse()

	args := pflag.Args()
	if len(args) == 0 {
		flag.Usage()
		return
	}

	var err error
	targetAddrs, err = convertAddrs(args[0])
	if err != nil {
		panic(err)
	}

	if *packetSize < len(msgPrefix)+8 {
		*packetSize = len(msgPrefix) + 8
	}

	log.SetFlags(log.Ltime)

	stat = NewBuckets()

	if err := start(); err != nil {
		panic(err)
	}
}
