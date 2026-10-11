// Copyright 2026 Abhinav Gautam
// SPDX-License-Identifier: Apache-2.0

package pmkusum

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Command is an annexure 7 exchange: an on-demand or config read or write
// from the platform, and (with the same handshake keywords) the RMS's
// answer. Params holds the annexure 7 keywords: for a read their values
// are ignored and the RMS replaces each; for a write they are the values
// to set.
type Command struct {
	Time   time.Time
	Type   string // TypeOnDemand or TypeConfig
	Cmd    string // "read" or "write"
	MsgID  string
	Params map[string]any
	// POTP and COTP are added when set (the annexure 7 samples carry
	// none; some platforms ask for them).
	POTP, COTP string
}

// handshake are the annexure 7 keywords every command carries.
var handshake = map[string]bool{"timestamp": true, "type": true, "cmd": true, "msgid": true}

// Read is a read of keys.
func Read(msgType, msgID string, t time.Time, keys ...string) Command {
	p := map[string]any{}
	for _, k := range keys {
		p[k] = ""
	}
	return Command{Time: t, Type: msgType, Cmd: "read", MsgID: msgID, Params: p}
}

// Write is a write of params.
func Write(msgType, msgID string, t time.Time, params map[string]any) Command {
	return Command{Time: t, Type: msgType, Cmd: "write", MsgID: msgID, Params: params}
}

// Check reports what makes a command unsendable.
func (c Command) Check() error {
	if c.Type != TypeOnDemand && c.Type != TypeConfig {
		return fmt.Errorf("type %q must be %s or %s", c.Type, TypeOnDemand, TypeConfig)
	}
	if c.Cmd != "read" && c.Cmd != "write" {
		return fmt.Errorf("cmd %q must be read or write", c.Cmd)
	}
	if c.MsgID == "" {
		return errors.New("msgid is required to match the answer")
	}
	if len(c.Params) == 0 {
		return errors.New("a command needs at least one keyword")
	}
	for k := range c.Params {
		if handshake[strings.ToLower(k)] {
			return fmt.Errorf("%s is a handshake keyword, not a parameter", k)
		}
	}
	return nil
}

// Encode writes the command as annexure 7 does: the handshake keywords in
// lower case, timestamp in loc as YYYY-MM-DD HH:mm:SS.
func (c Command) Encode(loc *time.Location) ([]byte, error) {
	if err := c.Check(); err != nil {
		return nil, err
	}
	if loc == nil {
		loc = time.UTC
	}
	m := map[string]any{}
	for k, v := range c.Params {
		m[k] = v
	}
	m["timestamp"] = c.Time.In(loc).Format("2006-01-02 15:04:05")
	m["type"], m["cmd"], m["msgid"] = c.Type, c.Cmd, c.MsgID
	if c.COTP != "" {
		m["POTP"], m["COTP"] = c.POTP, c.COTP
	}
	return json.Marshal(m)
}

// DecodeCommand reads an annexure 7 command (the RMS's side), with the
// handshake keywords in either case (section 6 prints them in upper case,
// annexure 7 in lower case). Values keep their JSON form (json.Number for
// numbers).
func DecodeCommand(payload []byte, loc *time.Location) (Command, error) {
	d := json.NewDecoder(bytes.NewReader(payload))
	d.UseNumber()
	var raw map[string]any
	if err := d.Decode(&raw); err != nil {
		return Command{}, fmt.Errorf("the command is not a JSON object: %w", err)
	}
	c := Command{Params: map[string]any{}}
	for k, v := range raw {
		switch strings.ToLower(k) {
		case "timestamp":
			if t, err := ParseTimestamp(text(v), loc); err == nil {
				c.Time = t
			}
		case "type":
			c.Type = strings.ToLower(text(v))
		case "cmd", "command":
			c.Cmd = strings.ToLower(text(v))
		case "msgid":
			c.MsgID = text(v)
		case "potp":
			c.POTP = text(v)
		case "cotp":
			c.COTP = text(v)
		default:
			c.Params[k] = v
		}
	}
	return c, c.Check()
}

// Answer builds the RMS's answer to c: the handshake keywords as received
// and each keyword's value from value (a read: the current value; a
// write: the value now in force). value reports false for a keyword the
// RMS does not know, which annexure 7 answers with 0.
func (c Command) Answer(value func(key string, written any) (any, bool), loc *time.Location) ([]byte, error) {
	if loc == nil {
		loc = time.UTC
	}
	m := map[string]any{"timestamp": c.Time.In(loc).Format("2006-01-02 15:04:05"), "type": c.Type, "cmd": c.Cmd, "msgid": c.MsgID}
	for k, v := range c.Params {
		if x, ok := value(k, v); ok {
			m[k] = x
		} else {
			m[k] = 0
		}
	}
	return json.Marshal(m)
}

// Result is what an RMS's answer says about a command.
type Result struct {
	// Values are the answered keywords (upper case), credentials masked.
	Values map[string]any
	// Invalid are asked keywords answered with 0 that no table knows
	// (annexure 7 command B).
	Invalid []string
	// NotApplied are written keywords answered with another value: a
	// controller in local mode or after a protection trip keeps its
	// output.
	NotApplied []string
	// Refused is set when the answer carries an explicit refusal
	// (Message.Ack false).
	Refused bool
}

// OK reports an answer with nothing invalid, refused or not applied.
func (r Result) OK() bool { return !r.Refused && len(r.Invalid) == 0 && len(r.NotApplied) == 0 }

// Err describes a failed result (nil when OK).
func (r Result) Err() error {
	switch {
	case r.Refused:
		return errors.New("the RMS refused the command")
	case len(r.Invalid) > 0:
		return fmt.Errorf("the RMS did not know %s", strings.Join(r.Invalid, ", "))
	case len(r.NotApplied) > 0:
		return fmt.Errorf("the RMS kept its value of %s", strings.Join(r.NotApplied, ", "))
	}
	return nil
}

// Evaluate compares an answer (decoded with Decode) with the command it
// answers. It does not check the msgid; match answers to commands first.
func Evaluate(sent Command, answer *Message) Result {
	res := Result{Values: map[string]any{}}
	if answer.Ack != nil && !*answer.Ack {
		res.Refused = true
	}
	asked := map[string]any{}
	for k, v := range sent.Params {
		asked[strings.ToUpper(k)] = v
	}
	for k, v := range answer.Raw {
		switch k {
		case "TIMESTAMP", "TYPE", "CMD", "MSGID", "COMMAND":
			continue
		}
		if SecretKeys[k] {
			res.Values[k] = "***"
		} else {
			res.Values[k] = plain(v)
		}
		want, ok := asked[k]
		if !ok {
			continue
		}
		if isZero(v) && !KnownKey(k) {
			res.Invalid = append(res.Invalid, k)
			continue
		}
		if sent.Cmd == "write" && !SecretKeys[k] && text(want) != "" && !sameValue(v, want) {
			res.NotApplied = append(res.NotApplied, k)
		}
	}
	slices.Sort(res.Invalid)
	slices.Sort(res.NotApplied)
	return res
}

func isZero(v any) bool {
	f, err := strconv.ParseFloat(text(v), 64)
	return err == nil && f == 0
}

// sameValue compares two keyword values as numbers when both are, else
// as text.
func sameValue(a, b any) bool {
	x, ea := strconv.ParseFloat(text(a), 64)
	y, eb := strconv.ParseFloat(text(b), 64)
	if ea == nil && eb == nil {
		return x == y
	}
	return text(a) == text(b)
}

// MaskSecrets returns params with the values of SecretKeys replaced.
func MaskSecrets(params map[string]any) map[string]any {
	out := make(map[string]any, len(params))
	for k, v := range params {
		if SecretKeys[strings.ToUpper(k)] {
			out[k] = "***"
		} else {
			out[k] = v
		}
	}
	return out
}

// Annexure 7 actions as ready commands (msgid and time are the caller's).

// Pump switches the pump through DO1 (1 on, 0 off). Section 3 calls remote
// operation an on-demand write while annexure 7 prints DO1 in a config
// message; msgType chooses (TypeOnDemand when empty).
func Pump(msgType, msgID string, t time.Time, on bool) Command {
	if msgType == "" {
		msgType = TypeOnDemand
	}
	return Write(msgType, msgID, t, map[string]any{"DO1": b2i(on)})
}

// Restart restarts the RMS (RESTART 1).
func Restart(msgID string, t time.Time) Command {
	return Write(TypeConfig, msgID, t, map[string]any{"RESTART": 1})
}

// Interval sets the periodic push interval in minutes (UPDATEINTERVAL;
// the MNRE RMS specification allows 1 to 15).
func Interval(msgID string, t time.Time, minutes int) (Command, error) {
	if minutes < 1 || minutes > 15 {
		return Command{}, errors.New("the push interval must be 1 to 15 minutes")
	}
	return Write(TypeConfig, msgID, t, map[string]any{"UPDATEINTERVAL": minutes}), nil
}

// HeartbeatInterval sets the heartbeat interval in minutes (HEARTINTERVAL).
func HeartbeatInterval(msgID string, t time.Time, minutes int) (Command, error) {
	if minutes < 1 || minutes > 1440 {
		return Command{}, errors.New("the heartbeat interval must be 1 to 1440 minutes")
	}
	return Write(TypeConfig, msgID, t, map[string]any{"HEARTINTERVAL": minutes}), nil
}

// SyncClock sets the RMS clock to now in loc (URTCDATE as YYMMDD, URTCTIME
// as HHmmSS, UPDATERTC 1).
func SyncClock(msgID string, t, now time.Time, loc *time.Location) Command {
	if loc == nil {
		loc = time.UTC
	}
	n := now.In(loc)
	d, _ := strconv.Atoi(n.Format("060102"))
	tm, _ := strconv.Atoi(n.Format("150405"))
	return Write(TypeConfig, msgID, t, map[string]any{"URTCDATE": d, "URTCTIME": tm, "UPDATERTC": 1})
}

// GSMSync asks the RMS to take its clock from the GSM network (GSMSYNC 1).
func GSMSync(msgID string, t time.Time) Command {
	return Write(TypeConfig, msgID, t, map[string]any{"GSMSYNC": 1})
}

// APN sets the access point of SIM slot 1 or 2 (APNn, USRn, PASSn).
func APN(msgID string, t time.Time, slot int, apn, user, pass string) (Command, error) {
	if slot != 1 && slot != 2 {
		return Command{}, errors.New("the SIM slot must be 1 or 2")
	}
	n := strconv.Itoa(slot)
	return Write(TypeConfig, msgID, t, map[string]any{"APN" + n: apn, "USR" + n: user, "PASS" + n: pass}), nil
}

// Broker moves the RMS to another MQTT server (URL, PORT, CID, USERNAME,
// PASSWORD).
func Broker(msgID string, t time.Time, host string, port int, clientID, user, pass string) Command {
	return Write(TypeConfig, msgID, t, map[string]any{"URL": host, "PORT": port, "CID": clientID, "USERNAME": user, "PASSWORD": pass})
}

// Firmware asks the RMS to fetch firmware over FTP (FTPURL, FTPUSER,
// FTPPASS, FTPPORT, FTPDOWN 1).
func Firmware(msgID string, t time.Time, host string, port int, user, pass string) Command {
	return Write(TypeConfig, msgID, t, map[string]any{"FTPURL": host, "FTPUSER": user, "FTPPASS": pass, "FTPPORT": port, "FTPDOWN": 1})
}

// AnalogCalibration sets analog input n's zero and span (AInZERO, AInSPAN;
// n is 1 to 4).
func AnalogCalibration(msgID string, t time.Time, n int, zero, span float64) (Command, error) {
	if n < 1 || n > 4 {
		return Command{}, errors.New("the analog input must be 1 to 4")
	}
	s := strconv.Itoa(n)
	return Write(TypeConfig, msgID, t, map[string]any{"AI" + s + "ZERO": zero, "AI" + s + "SPAN": span}), nil
}

// Retrieve asks for one stored record again: an on-demand write of VD,
// DATE (YYMMDD), INDEX and LOAD 1. Section 3 names a history push mode but
// no request; this form is ours (PROVENANCE.md).
func Retrieve(msgID string, t time.Time, vd int, date string, index int) (Command, error) {
	if vd < 0 || index < 0 {
		return Command{}, errors.New("vd and index must not be negative")
	}
	ds, err := YYMMDD(date)
	if err != nil {
		return Command{}, err
	}
	n, _ := strconv.Atoi(ds)
	return Write(TypeOnDemand, msgID, t, map[string]any{"VD": vd, "DATE": n, "INDEX": index, "LOAD": 1}), nil
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}
