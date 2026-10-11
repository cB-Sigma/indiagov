// Copyright 2026 Abhinav Gautam
// SPDX-License-Identifier: Apache-2.0

package pmkusum

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// One-time passwords. The profile has the platform publish an OTP pair on
// the otp topic (POTP the previous, COTP the current) every 15, 30 or 60
// minutes, and has every RMS message carry it. It defines neither the otp
// message's layout nor how the value is made, so OTPMessage and OTPSource
// are this module's choice, kept apart from the profile's own keywords.

// OTPMessage is the otp topic's message as this module writes it:
// {"TIMESTAMP", "IMEI", "POTP", "COTP", "VALIDITY"} (VALIDITY in minutes).
type OTPMessage struct {
	Time       time.Time
	IMEI       string
	POTP, COTP string
	Validity   time.Duration
}

// Encode writes the message; TIMESTAMP is in loc.
func (o OTPMessage) Encode(loc *time.Location) ([]byte, error) {
	if o.COTP == "" {
		return nil, errors.New("COTP is required")
	}
	if loc == nil {
		loc = time.UTC
	}
	m := map[string]any{"TIMESTAMP": o.Time.In(loc).Format("2006-01-02 15:04:05"), "IMEI": o.IMEI, "POTP": o.POTP, "COTP": o.COTP}
	if o.Validity > 0 {
		m["VALIDITY"] = int(o.Validity / time.Minute)
	}
	return json.Marshal(m)
}

// DecodeOTP reads an otp message: POTP and COTP from any JSON object (keys
// in any case), the other members when present.
func DecodeOTP(payload []byte, loc *time.Location) (OTPMessage, error) {
	d := json.NewDecoder(bytes.NewReader(payload))
	d.UseNumber()
	var raw map[string]any
	if err := d.Decode(&raw); err != nil {
		return OTPMessage{}, fmt.Errorf("the OTP message is not a JSON object: %w", err)
	}
	m := Message{Raw: map[string]any{}}
	for k, v := range raw {
		m.Raw[upper(k)] = v
	}
	o := OTPMessage{IMEI: text(m.Raw["IMEI"]), POTP: text(m.Raw["POTP"]), COTP: text(m.Raw["COTP"])}
	if o.COTP == "" {
		return o, errors.New("the OTP message has no COTP")
	}
	if ts := text(m.Raw["TIMESTAMP"]); ts != "" {
		if t, err := ParseTimestamp(ts, loc); err == nil {
			o.Time = t
		}
	}
	if v := intp(m.Raw["VALIDITY"]); v != nil {
		o.Validity = time.Duration(*v) * time.Minute
	}
	return o, nil
}

// OTPSource makes each RMS's OTPs without keeping state: the OTP of a
// window is an HMAC-SHA256 of the IMEI and the window number under
// Secret, cut to Digits decimal digits by the dynamic truncation of RFC
// 4226 clause 5.3. Every server holding the secret computes the same
// values.
type OTPSource struct {
	Secret []byte
	// Interval is the window (default 30 minutes).
	Interval time.Duration
	// Digits is the OTP length (default 8; the samples print 6 and 8).
	Digits int
}

func (s OTPSource) interval() time.Duration {
	if s.Interval <= 0 {
		return 30 * time.Minute
	}
	return s.Interval
}

func (s OTPSource) digits() int {
	if s.Digits <= 0 {
		return 8
	}
	return s.Digits
}

// Window is the number of the window holding t.
func (s OTPSource) Window(t time.Time) int64 { return t.Unix() / int64(s.interval()/time.Second) }

// WindowStart is when the window holding t began.
func (s OTPSource) WindowStart(t time.Time) time.Time {
	return time.Unix(s.Window(t)*int64(s.interval()/time.Second), 0)
}

// OTP is imei's OTP for window w.
func (s OTPSource) OTP(imei string, w int64) string {
	mac := hmac.New(sha256.New, s.Secret)
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(w))
	mac.Write([]byte(imei))
	mac.Write([]byte{0})
	mac.Write(b[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	v := uint64(binary.BigEndian.Uint32(sum[off:off+4]) & 0x7fffffff)
	mod := uint64(1)
	for i := 0; i < s.digits(); i++ {
		mod *= 10
	}
	return fmt.Sprintf("%0*d", s.digits(), v%mod)
}

// Pair is imei's previous and current OTP at t.
func (s OTPSource) Pair(imei string, t time.Time) (potp, cotp string) {
	w := s.Window(t)
	return s.OTP(imei, w-1), s.OTP(imei, w)
}

// Message is the otp message for imei at t.
func (s OTPSource) Message(imei string, t time.Time) OTPMessage {
	p, c := s.Pair(imei, t)
	return OTPMessage{Time: t, IMEI: imei, POTP: p, COTP: c, Validity: s.interval()}
}

// Valid reports whether cotp is imei's OTP for the window holding t or
// the one before (an RMS that has not yet received the new pair).
func (s OTPSource) Valid(imei, cotp string, t time.Time) bool {
	if cotp == "" {
		return false
	}
	w := s.Window(t)
	for _, x := range []int64{w, w - 1} {
		if subtle.ConstantTimeCompare([]byte(s.OTP(imei, x)), []byte(cotp)) == 1 {
			return true
		}
	}
	return false
}
