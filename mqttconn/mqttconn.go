// Copyright 2026 Abhinav Gautam
// SPDX-License-Identifier: Apache-2.0

// Package mqttconn is the small MQTT seam of the simulators in this
// module: a Dialer opens a reconnecting session that subscribes to
// filters and publishes. Paho is an MQTT 5 implementation on Eclipse
// Paho's Go client (github.com/eclipse/paho.golang); an application with
// its own MQTT client supplies its own Dialer instead.
package mqttconn

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
)

// Message is a received message.
type Message struct {
	Topic   string
	Payload []byte
}

// Options shape a session.
type Options struct {
	// NoLocal asks the broker not to send the session its own messages
	// (MQTT 5 No Local).
	NoLocal bool
	// SessionExpiry, when positive, keeps the broker session for this long
	// after the connection is lost, so QoS 1 messages published meanwhile
	// are delivered on reconnect.
	SessionExpiry time.Duration
	// OnUp runs after each connection, once the subscriptions are in place.
	OnUp func(Conn)
}

// Conn is an open session.
type Conn interface {
	Publish(ctx context.Context, topic string, payload []byte, retain bool) error
	Close()
}

// Dialer opens a session on the broker at url ("mqtt://host:1883"), with
// clientID, subscribed to filters; handle gets each message. It returns
// once the first connection is up or ctx ends.
type Dialer func(ctx context.Context, url, clientID string, filters []string, handle func(Message), opts Options) (Conn, error)

type pahoConn struct {
	cm     *autopaho.ConnectionManager
	cancel context.CancelFunc
	once   sync.Once
}

func (c *pahoConn) Publish(ctx context.Context, topic string, payload []byte, retain bool) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := c.cm.AwaitConnection(ctx); err != nil {
		return err
	}
	_, err := c.cm.Publish(ctx, &paho.Publish{Topic: topic, QoS: 1, Retain: retain, Payload: payload})
	return err
}

func (c *pahoConn) Close() {
	c.once.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = c.cm.Disconnect(ctx)
		c.cancel()
	})
}

// Paho is a Dialer on Eclipse Paho's MQTT 5 client (QoS 1, automatic
// reconnection, subscriptions restored on every connection).
func Paho(ctx context.Context, rawURL, clientID string, filters []string, handle func(Message), opts Options) (Conn, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("MQTT broker %q: %w", rawURL, err)
	}
	switch u.Scheme {
	case "tcp":
		u.Scheme = "mqtt"
	case "ssl", "tls":
		u.Scheme = "mqtts"
	}
	if u.Port() == "" {
		if u.Scheme == "mqtts" {
			u.Host += ":8883"
		} else {
			u.Host += ":1883"
		}
	}
	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	c := &pahoConn{cancel: cancel}
	var expiry uint32
	if opts.SessionExpiry > 0 {
		expiry = uint32((opts.SessionExpiry + time.Second - 1) / time.Second)
	}
	cfg := autopaho.ClientConfig{
		ServerUrls:                    []*url.URL{u},
		KeepAlive:                     30,
		CleanStartOnInitialConnection: opts.SessionExpiry <= 0,
		SessionExpiryInterval:         expiry,
		ConnectTimeout:                10 * time.Second,
		ReconnectBackoff:              pacedBackoff(autopaho.NewConstantBackoff(10 * time.Second)),
		OnConnectionUp: func(cm *autopaho.ConnectionManager, _ *paho.Connack) {
			go func() {
				if len(filters) > 0 {
					s := &paho.Subscribe{}
					for _, f := range filters {
						s.Subscriptions = append(s.Subscriptions, paho.SubscribeOptions{Topic: f, QoS: 1,
							NoLocal: opts.NoLocal && !strings.HasPrefix(f, "$share/")})
					}
					sctx, scancel := context.WithTimeout(runCtx, 10*time.Second)
					_, _ = cm.Subscribe(sctx, s)
					scancel()
				}
				if opts.OnUp != nil {
					opts.OnUp(c)
				}
			}()
		},
		ClientConfig: paho.ClientConfig{
			ClientID: clientID,
			OnPublishReceived: []func(paho.PublishReceived) (bool, error){
				func(pr paho.PublishReceived) (bool, error) {
					handle(Message{Topic: pr.Packet.Topic, Payload: pr.Packet.Payload})
					return true, nil
				},
			},
		},
	}
	if u.User != nil {
		cfg.ConnectUsername = u.User.Username()
		if p, ok := u.User.Password(); ok {
			cfg.ConnectPassword = []byte(p)
		}
	}
	cm, err := autopaho.NewConnection(runCtx, cfg)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("MQTT broker %s: %w", rawURL, err)
	}
	c.cm = cm
	if err := cm.AwaitConnection(ctx); err != nil {
		cancel()
		return nil, fmt.Errorf("MQTT broker %s: %w", rawURL, err)
	}
	return c, nil
}

// pacedBackoff wraps an autopaho back-off, which covers only failed
// attempts: autopaho dials again at once after a connection ends (attempt
// 0), so a server that accepts the session and drops it straight away
// would be dialled in a busy loop. Attempt 0 now starts at least a second
// after the attempt before; after a connection that lasted, it is made at
// once.
func pacedBackoff(b autopaho.Backoff) autopaho.Backoff {
	var mu sync.Mutex
	var last time.Time // when the latest attempt began
	return func(attempt int) time.Duration {
		d := b(attempt)
		mu.Lock()
		defer mu.Unlock()
		now := time.Now()
		if attempt == 0 && !last.IsZero() {
			d = max(d, time.Second-now.Sub(last))
		}
		last = now.Add(d)
		return d
	}
}
