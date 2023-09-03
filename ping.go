package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
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

	for _, target := range strings.Split(srcAddr, ",") {
		validTargets[target] = true
	}
}

var connOnce sync.Once

func start() error {
	localIP, err := qianmo.HostFirstIPv4()
	if err != nil {
		return err
	}
	iface, err := qianmo.InterfaceByIP(localIP)
	if err != nil {
		return err
	}

	conn, err := icmpx.ListenIPv4(iface, icmpx.IPv4Config{
		Filter: icmpx.IPv4AllowOnly(ipv4.ICMPTypeEchoReply),
	})
	if err != nil {
		return err
	}

	go send(conn)

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
	binary.LittleEndian.PutUint64(data[len(msgPrefix):], uint64(time.Now().UnixNano()))

	_, err := rand.Read(data[len(msgPrefix):])
	if err != nil {
		panic(err)
	}

	for {
		seq++
		req := &icmp.Message{
			Type: ipv4.ICMPTypeEcho,
			Body: &icmp.Echo{
				ID:   int(id),
				Seq:  int(seq),
				Data: []byte{},
			},
		}

		limiter.Take()
		for _, target := range targetAddrs {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			if err := conn.WriteTo(ctx, req, target); err != nil {
				fmt.Println(err)
				return
			}

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

		if !validTargets[addr.String()] {
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

			fmt.Printf("%d bytes from %s: icmp_seq=%d ttl=%v\n", len(pkt.Data), addr, pkt.Seq, time.Now().Sub(time.Unix(0, ts)))
		}
	}
}
