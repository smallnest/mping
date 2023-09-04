package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/netip"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/mdlayher/icmpx"
	"github.com/smallnest/qianmo"
	"go.uber.org/ratelimit"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

var id uint16
var validTargets = make(map[string]bool)

func init() {
	rand.New(rand.NewSource(time.Now().UnixNano()))
	id = uint16(os.Getpid() & 0xffff)

}

var connOnce sync.Once

func start() error {
	for _, target := range strings.Split(srcAddr, ",") {
		validTargets[target] = true
	}

	addrs := qianmo.NonLoopbackAddrs()
	if len(addrs) == 0 {
		return errors.New("no non-loopback address")
	}

	iface, err := qianmo.InterfaceByIP(addrs[0])
	if err != nil {
		return fmt.Errorf("failed to get interface by ip: %w", err)
	}

	conn, err := icmpx.ListenIPv4(iface, icmpx.IPv4Config{
		Filter: icmpx.IPv4AllowOnly(ipv4.ICMPTypeEchoReply),
	})
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	go send(conn)
	go printStat()

	return read(conn)
}

func send(conn *icmpx.IPv4Conn) {
	defer connOnce.Do(func() { conn.Close() })

	limiter := ratelimit.New(*rate, ratelimit.Per(time.Second))

	targets := strings.Split(srcAddr, ",")
	var targetAddrs []netip.Addr
	for _, target := range targets {
		targetAddrs = append(targetAddrs, netip.MustParseAddr(target))
	}

	var seq uint16

	data := make([]byte, *packetSize)
	copy(data, msgPrefix)

	_, err := rand.Read(data[len(msgPrefix)+8:])
	if err != nil {
		panic(err)
	}

	sentPackets := 0
	for {
		seq++
		ts := time.Now().UnixNano()
		binary.LittleEndian.PutUint64(data[len(msgPrefix):], uint64(ts))

		req := &icmp.Message{
			Type: ipv4.ICMPTypeEcho,
			Body: &icmp.Echo{
				ID:   int(id),
				Seq:  int(seq),
				Data: data,
			},
		}

		limiter.Take()
		for _, target := range targetAddrs {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			key := ts / int64(time.Second)
			stat.Add(key, &Result{
				ts:     ts,
				target: target.String(),
				seq:    seq,
			})

			if err := conn.WriteTo(ctx, req, target); err != nil {
				return
			}
		}

		sentPackets++
		if *count > 0 && sentPackets >= *count {
			fmt.Printf("sent packets: %d\n", sentPackets)
			return
		}
	}
}

func read(conn *icmpx.IPv4Conn) error {
	defer connOnce.Do(func() { conn.Close() })

	for {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		msg, addr, err := conn.ReadFrom(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}

		target := addr.String()
		if !validTargets[target] {
			continue
		}

		switch pkt := msg.Body.(type) {
		case *icmp.Echo:
			if uint16(pkt.ID) != id {
				continue
			}

			if len(pkt.Data) < len(msgPrefix)+8 {
				continue
			}

			if !bytes.HasPrefix(pkt.Data, msgPrefix) {
				continue
			}

			ts := int64(binary.LittleEndian.Uint64(pkt.Data[len(msgPrefix):]))
			key := ts / int64(time.Second)
			stat.Add(key, &Result{
				ts:       ts,
				target:   target,
				ttl:      time.Now().UnixNano() - ts,
				received: true,
				seq:      uint16(pkt.Seq),
			})
		}
	}
}

func printStat() {
	delayInSeconds := int64(*delay) // 5s
	ticker := time.NewTicker(time.Second)
	var lastKey int64

	for range ticker.C {
	recheck:
		bucket := stat.Last()
		if bucket == nil {
			continue
		}

		// fmt.Println("lastKey:", lastKey, "bucket.Key:", bucket.Key)

		if bucket.Key <= lastKey {
			stat.Pop()
			goto recheck
		}

		if bucket.Key <= time.Now().UnixNano()/int64(time.Second)-delayInSeconds {
			pop := stat.Pop().(*Bucket)
			if pop.Key < bucket.Key {
				goto recheck
			}

			lastKey = pop.Key

			targetResult := make(map[string]*TargetResult)

			for _, r := range pop.Value {
				target := r.target

				tr := targetResult[target]
				if tr == nil {
					tr = &TargetResult{}
					targetResult[target] = tr
				}

				tr.ttl += r.ttl

				if r.received {
					tr.received++
				} else {
					tr.loss++
				}

			}

			for target, tr := range targetResult {
				total := tr.received + tr.loss
				var lossRate float64
				if total == 0 {
					lossRate = 0
				} else {
					lossRate = float64(tr.loss) / float64(total)
				}

				if tr.received == 0 {
					log.Printf("%s: sent:%d, recv:%d, loss rate: %.2f%%, ttl: %v\n", target, total, tr.loss, lossRate*100, 0)
				} else {
					log.Printf("%s: sent:%d, recv:%d,  loss rate: %.2f%%, ttl: %v\n", target, total, tr.loss, lossRate*100, time.Duration(tr.ttl/int64(tr.received)))
				}

			}

		}
	}

}
