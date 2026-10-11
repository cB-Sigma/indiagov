// Copyright 2026 Abhinav Gautam
// SPDX-License-Identifier: Apache-2.0

package mqttconn_test

import (
	"bufio"
	"context"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cB-Sigma/indiagov/mqttconn"
)

// A broker that accepts the session and drops it at once must not be
// dialled in a busy loop.
func TestPahoPacesABrokerThatDropsAtOnce(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	var n atomic.Int64
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			n.Add(1)
			go dropAfterConnack(c)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := mqttconn.Paho(ctx, "mqtt://"+ln.Addr().String(), "pace-test", nil, func(mqttconn.Message) {}, mqttconn.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	start := n.Load()
	time.Sleep(2 * time.Second)
	if got := n.Load() - start; got > 5 {
		t.Errorf("%d connections in 2s to a broker that drops at once; want a paced few", got)
	}
}

// dropAfterConnack reads the CONNECT, answers success (MQTT 5) and closes.
func dropAfterConnack(c net.Conn) {
	defer c.Close()
	r := bufio.NewReader(c)
	if _, err := r.ReadByte(); err != nil {
		return
	}
	size, mul := 0, 1
	for {
		b, err := r.ReadByte()
		if err != nil {
			return
		}
		size += int(b&0x7f) * mul
		if b&0x80 == 0 {
			break
		}
		mul *= 128
	}
	if _, err := io.CopyN(io.Discard, r, int64(size)); err != nil {
		return
	}
	_, _ = c.Write([]byte{0x20, 0x03, 0x00, 0x00, 0x00})
}
