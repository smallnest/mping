package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"go.uber.org/ratelimit"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/sys/unix"
)

var (
	// ErrStampNotFund is returned when timestamp not found
	ErrStampNotFund = errors.New("timestamp not found")
)

var id uint16
var validTargets = make(map[string]bool)
var supportTxTimestamping = true
var supportRxTimestamping = true

func init() {
	rand.New(rand.NewSource(time.Now().UnixNano()))
	id = uint16(os.Getpid() & 0xffff)

}

var connOnce sync.Once

func start() error {
	for _, target := range targetAddrs {
		validTargets[target] = true
	}

	if len(targetAddrs) == 0 {
		return errors.New("no target")
	}

	conn, err := openConn()
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	if *tos > 0 {
		pconn := ipv4.NewConn(conn)
		err = pconn.SetTOS(*tos)
		if err != nil {
			return fmt.Errorf("failed to set tos: %w", err)
		}
	}

	done := make(chan error, 3)
	go func() {
		err := send(conn)
		done <- err
	}()

	go func() {
		err := printStat()
		done <- err
	}()
	go func() {
		read(conn)
		done <- err
	}()

	return <-done
}

func openConn() (*net.IPConn, error) {
	conn, err := net.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return nil, err
	}

	ipconn := conn.(*net.IPConn)
	f, err := ipconn.File()
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fd := int(f.Fd())
	// https://patchwork.ozlabs.org/project/netdev/patch/1226415412.31699.2.camel@ecld0pohly/
	// https://www.kernel.org/doc/Documentation/networking/timestamping.txt
	flags := unix.SOF_TIMESTAMPING_SYS_HARDWARE | unix.SOF_TIMESTAMPING_RAW_HARDWARE | unix.SOF_TIMESTAMPING_SOFTWARE | unix.SOF_TIMESTAMPING_RX_HARDWARE | unix.SOF_TIMESTAMPING_RX_SOFTWARE |
		unix.SOF_TIMESTAMPING_TX_HARDWARE | unix.SOF_TIMESTAMPING_TX_SOFTWARE |
		unix.SOF_TIMESTAMPING_OPT_CMSG | unix.SOF_TIMESTAMPING_OPT_TSONLY
	if err := syscall.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_TIMESTAMPING, flags); err != nil {
		supportTxTimestamping = false
		supportRxTimestamping = false
		if err := syscall.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_TIMESTAMPNS, 1); err == nil {
			supportRxTimestamping = true
		}

		return ipconn, nil
	}
	timeout := syscall.Timeval{Sec: 1, Usec: 0}
	if err := syscall.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &timeout); err != nil {
		return nil, err
	}
	if err := syscall.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_SNDTIMEO, &timeout); err != nil {
		return nil, err
	}

	return ipconn, nil
}

var payload []byte

func send(conn *net.IPConn) error {
	defer connOnce.Do(func() { conn.Close() })
	f, err := conn.File()
	if err != nil {
		return err
	}
	defer f.Close()
	fd := int(f.Fd())

	limiter := ratelimit.New(*rate, ratelimit.Per(time.Second))

	var seq uint16

	data := make([]byte, *packetSize)
	copy(data, msgPrefix)

	_, err = rand.Read(data[len(msgPrefix)+8:])
	if err != nil {
		return err
	}

	payload = data[len(msgPrefix)+8:]

	targets := make([]*net.IPAddr, 0, len(targetAddrs))
	for _, taget := range targetAddrs {
		targets = append(targets, &net.IPAddr{IP: net.ParseIP(taget)})
	}

	sentPackets := 0
	for {
		seq++

		limiter.Take()
		for _, target := range targets {
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

			key := ts / int64(time.Second)

			data, err := req.Marshal(nil)
			if err != nil {
				continue
			}
			_, err = conn.WriteTo(data, target)
			if err != nil {
				return err
			}

			rs := &Result{
				txts:   ts,
				target: target.IP.String(),
				seq:    seq,
			}

			if supportTxTimestamping {
				if txts, err := getTxTs(fd); err != nil {
					if strings.HasPrefix(err.Error(), "resource temporarily unavailable") {
						continue
					}
					fmt.Printf("failed to get TX timestamp: %s", err)
					rs.txts = txts
				}
			}

			stat.Add(key, rs)

		}

		sentPackets++
		if *count > 0 && sentPackets >= *count {
			time.Sleep(time.Second * time.Duration((*delay + 1)))
			fmt.Printf("sent packets: %d\n", sentPackets)
			return nil
		}

	}
}

func read(conn *net.IPConn) error {
	// defer func() {
	// 	if err := recover(); err != nil {
	// 		// fmt.Println(err)
	// 	}
	// }()
	defer connOnce.Do(func() { conn.Close() })

	pktBuf := make([]byte, 1500)
	oob := make([]byte, 1500)

	for {
		_ = conn.SetReadDeadline(time.Now().Add(10 * time.Millisecond))
		n, oobn, _, ra, err := conn.ReadMsgIP(pktBuf, oob)

		if err != nil {
			// if neterr, ok := err.(*net.OpError); ok && neterr.Timeout() {
			// 	return nil
			// }
			// if strings.Contains(err.Error(), "no message of desired type") {
			// 	return nil
			// }
			return err
		}

		var rxts int64
		if supportRxTimestamping {
			if rxts, err = getTsFromOOB(oob, oobn); err != nil {
				return fmt.Errorf("failed to get RX timestamp: %s", err)
			}
		} else {
			rxts = time.Now().UnixNano()
		}

		if n < ipv4.HeaderLen {
			return errors.New("malformed IPv4 packet")
		}

		target := ra.String()
		if !validTargets[target] {
			continue
		}

		msg, err := icmp.ParseMessage(1, pktBuf[ipv4.HeaderLen:])
		if err != nil {
			continue
		}
		if msg.Type != ipv4.ICMPTypeEchoReply {
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

			txts := int64(binary.LittleEndian.Uint64(pkt.Data[len(msgPrefix):]))
			key := txts / int64(time.Second)

			bitflip := false
			if *bitflipCheck {
				bitflip = !bytes.Equal(pkt.Data[len(msgPrefix)+8:], payload)
			}

			stat.AddReply(key, &Result{
				txts:     txts,
				rxts:     rxts,
				target:   target,
				latency:  time.Now().UnixNano() - txts,
				received: true,
				seq:      uint16(pkt.Seq),
				bitflip:  bitflip,
			})
		}
	}
}

func printStat() error {
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

				tr.latency += r.latency

				if r.received {
					tr.received++
				} else {
					tr.loss++
				}

				if *bitflipCheck && r.bitflip {
					tr.bitflipCount++
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

				if *bitflipCheck {
					if tr.received == 0 {
						log.Printf("%s: sent:%d, recv:%d, loss rate: %.2f%%, latency: %v, bitflip: %d\n", target, total, tr.received, lossRate*100, 0, tr.bitflipCount)
					} else {
						log.Printf("%s: sent:%d, recv:%d,  loss rate: %.2f%%, latency: %v, bitflip: %d\n", target, total, tr.received, lossRate*100, time.Duration(tr.latency/int64(tr.received)), tr.bitflipCount)
					}
				} else {

					if tr.received == 0 {
						log.Printf("%s: sent:%d, recv:%d, loss rate: %.2f%%, latency: %v\n", target, total, tr.received, lossRate*100, 0)
					} else {
						log.Printf("%s: sent:%d, recv:%d,  loss rate: %.2f%%, latency: %v\n", target, total, tr.received, lossRate*100, time.Duration(tr.latency/int64(tr.received)))
					}
				}
			}

		}
	}

	return nil
}

func getTsFromOOB(oob []byte, oobn int) (int64, error) {
	cms, err := syscall.ParseSocketControlMessage(oob[:oobn])
	if err != nil {
		return 0, err
	}
	for _, cm := range cms {
		if cm.Header.Level == syscall.SOL_SOCKET && cm.Header.Type == syscall.SO_TIMESTAMPING {
			var t unix.ScmTimestamping
			if err := binary.Read(bytes.NewBuffer(cm.Data), binary.LittleEndian, &t); err != nil {
				return 0, err
			}

			for i := 0; i < len(t.Ts); i++ {
				if t.Ts[i].Nano() > 0 {
					return t.Ts[i].Nano(), nil
				}
			}

			return 0, ErrStampNotFund
		}

		if cm.Header.Level == syscall.SOL_SOCKET && cm.Header.Type == syscall.SCM_TIMESTAMPNS {
			var t unix.Timespec
			if err := binary.Read(bytes.NewBuffer(cm.Data), binary.LittleEndian, &t); err != nil {
				return 0, err
			}
			return t.Nano(), nil
		}
	}
	return 0, ErrStampNotFund
}

func getTxTs(socketFd int) (int64, error) {
	pktBuf := make([]byte, 1024)
	oob := make([]byte, 1024)
	_, oobn, _, _, err := syscall.Recvmsg(socketFd, pktBuf, oob, syscall.MSG_ERRQUEUE)
	if err != nil {
		return 0, err
	}
	return getTsFromOOB(oob, oobn)
}
