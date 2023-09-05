package main

import (
	"flag"
	"log"
	"os"
	"time"
)

var (
	count      = flag.Int("c", 0, "count, 0 means non-setting")
	tos        = flag.Int("z", 0, "tos, 0 means non-setting")
	packetSize = flag.Int("l", 64, "packet size")
	timeout    = flag.Duration("t", time.Second, "timeout")
	rate       = flag.Int("r", 100, "rate, 100 means 100 packets per second")
	delay      = flag.Int("d", 3, "delay seconds")
)

var (
	msgPrefix   = []byte("smallnest")
	targetAddrs []string
	stat        *buckets
)

func hasFlag(f string) bool {
	// 遍历命令行参数，检查是否存在 -t 参数
	for _, arg := range os.Args {
		if arg == f {
			return true
		}
	}
	return false
}

func main() {
	flag.Parse()

	args := flag.Args()
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
