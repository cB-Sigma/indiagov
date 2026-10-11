// Copyright 2026 Abhinav Gautam
// SPDX-License-Identifier: Apache-2.0

package pmkusum_test

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/cB-Sigma/indiagov/pmkusum"
)

var ist = time.FixedZone("IST", 19800)

func TestTopics(t *testing.T) {
	if got := pmkusum.Topic("", "Standalonesolarpump", "860000000000001", pmkusum.TypeData); got != "IIOT-1/Standalonesolarpump/860000000000001/data" {
		t.Errorf("Topic = %s", got)
	}
	cases := []struct {
		topic string
		want  pmkusum.Route
		ok    bool
	}{
		{"IIOT-1/Standalonesolarpump/860000000000001/Info", pmkusum.Route{Prefix: "IIOT-1", Solution: "Standalonesolarpump", IMEI: "860000000000001", Type: "info"}, true},
		{"iiot-1/standalonesolarpump/860000000000001/ondemand/pub", pmkusum.Route{Prefix: "iiot-1", Solution: "standalonesolarpump", IMEI: "860000000000001", Type: "ondemand", Direction: "pub"}, true},
		{"devices/860000000000001/messages/events/mtd=data&ver=iiot-1&app=standalonesolarpump", pmkusum.Route{Prefix: "iiot-1", Solution: "standalonesolarpump", IMEI: "860000000000001", Type: "data", Hub: true}, true},
		{"devices/860000000000001/messages/devicebound/x", pmkusum.Route{IMEI: "860000000000001", Type: "otp", Direction: "sub", Hub: true}, true},
		{"IIOT-1/S/860000000000001/data/extra", pmkusum.Route{}, false},
		{"IIOT-1//860000000000001/data", pmkusum.Route{}, false},
		{"IIOT-1/S/data", pmkusum.Route{}, false},
	}
	for _, c := range cases {
		got, ok := pmkusum.ParseTopic(c.topic)
		if ok != c.ok || got != c.want {
			t.Errorf("ParseTopic(%s) = %+v, %v; want %+v, %v", c.topic, got, ok, c.want, c.ok)
		}
	}
	hub := pmkusum.HubTopic("", "Standalonesolarpump", "860000000000001", pmkusum.TypeHeartbeat)
	if r, ok := pmkusum.ParseTopic(hub); !ok || r.Type != pmkusum.TypeHeartbeat || r.Solution != "standalonesolarpump" {
		t.Errorf("HubTopic %s read back as %+v", hub, r)
	}
}

// A message the RMS side writes with Encode decodes back to the same
// values on the platform's side.
func TestEncodeDecode(t *testing.T) {
	at := time.Date(2026, 10, 11, 9, 30, 0, 0, ist)
	kw := map[string]any{}
	for name, v := range map[string]any{"outputPower": "4.50", "runStatus": 1, "todayWater": 1200} {
		k, ok := pmkusum.Keyword(pmkusum.GroupPump, name, 2)
		if !ok {
			t.Fatalf("no keyword for %s", name)
		}
		kw[k] = v
	}
	asn, _ := pmkusum.AssetKeyword(pmkusum.GroupPump, 2)
	b, err := pmkusum.Encode(pmkusum.Header{VD: 1, Time: at, MaxIndex: 96, Index: 38, StInterval: 15, IMEI: "860000000000001",
		POTP: "11111111", COTP: "22222222", Asset: asn, AssetValue: "DEMO-PC-2"}, ist, kw)
	if err != nil {
		t.Fatal(err)
	}
	m, err := pmkusum.Decode(b, ist)
	if err != nil {
		t.Fatal(err)
	}
	if m.Group != pmkusum.GroupPump || m.Instance != 2 || m.Asset != "DEMO-PC-2" || !m.Time.Equal(at) || m.StorageDate != "2026-10-11" || m.COTP != "22222222" {
		t.Errorf("decoded %+v", m)
	}
	if m.Values["outputPower"] != 4.5 || m.Values["runStatus"] != 1.0 || m.Values["todayWater"] != 1200.0 {
		t.Errorf("values %v", m.Values)
	}
	if c := m.Content(); c["runStatusText"] != "running" || c["instance"] != 2 {
		t.Errorf("content %v", c)
	}
	if _, err := pmkusum.Encode(pmkusum.Header{IMEI: "1"}, ist, map[string]any{"imei": "2"}); err == nil {
		t.Error("a keyword that repeats a header keyword was taken")
	}
}

// Keywords in lower case, numbers as text and a TIMESTAMP with a T are
// read; an unknown keyword is kept in Other.
func TestDecodeLenient(t *testing.T) {
	m, err := pmkusum.Decode([]byte(`{"vd":0,"timestamp":"2026-10-11T09:30:00","imei":"860000000000001","gsm":"1","rssi":"21","vbatt":"4.1","XYZ":5}`), ist)
	if err != nil {
		t.Fatal(err)
	}
	if m.Group != pmkusum.GroupRMS || !m.HasTime || m.Values["rssi"] != 21.0 || m.Other["XYZ"] != 5.0 || m.Container() != "heartbeat" {
		t.Errorf("decoded %+v", m)
	}
	if _, err := pmkusum.Decode([]byte(`{"VD":1,`), ist); err == nil {
		t.Error("truncated JSON was taken")
	}
	m, _ = pmkusum.Decode([]byte(`{"TIMESTAMP":"yesterday","IMEI":"1"}`), ist)
	if m.HasTime || m.TimeErr == nil {
		t.Errorf("a bad TIMESTAMP: %+v", m)
	}
}

// Forms seen in field captures published on GitHub (facts only; our own
// values): OTPs sent as numbers, STINTERVAL with decimals, ASN_0 with one
// digit for the RMS itself.
func TestDecodeFieldForms(t *testing.T) {
	m, err := pmkusum.Decode([]byte(`{"VD":0,"TIMESTAMP":"2026-10-11 09:30:00","MAXINDEX":15,"INDEX":15,"LOAD":0,"STINTERVAL":5.00,"MSGID":"",
"DATE":261011,"COTP":123456,"POTP":654321,"IMEI":"860000000000001","ASN_0":"demo0001","GSM":1,"RSSI":11,"VBATT":5}`), ist)
	if err != nil {
		t.Fatal(err)
	}
	if m.COTP != "123456" || m.POTP != "654321" || *m.StInterval != 5 || m.Group != pmkusum.GroupRMS || m.Asset != "demo0001" || m.Values["rssi"] != 11.0 {
		t.Errorf("decoded %+v", m)
	}
}

func TestCommandRoundTrip(t *testing.T) {
	at := time.Date(2026, 10, 11, 9, 30, 0, 0, ist)
	c, err := pmkusum.Interval("41", at, 10)
	if err != nil {
		t.Fatal(err)
	}
	b, err := c.Encode(ist)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	_ = json.Unmarshal(b, &raw)
	if raw["timestamp"] != "2026-10-11 09:30:00" || raw["type"] != "config" || raw["cmd"] != "write" || raw["msgid"] != "41" || raw["UPDATEINTERVAL"] != 10.0 {
		t.Errorf("encoded %s", b)
	}
	got, err := pmkusum.DecodeCommand(b, ist)
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != c.Type || got.Cmd != c.Cmd || got.MsgID != c.MsgID || !got.Time.Equal(at) || len(got.Params) != 1 {
		t.Errorf("decoded %+v", got)
	}
	// Section 6 spells the handshake in upper case.
	if got, err := pmkusum.DecodeCommand([]byte(`{"TIMESTAMP":"2026-10-11 09:30:00","TYPE":"ondemand","COMMAND":"read","MSGID":7,"RSSI":""}`), ist); err != nil || got.Cmd != "read" || got.MsgID != "7" {
		t.Errorf("upper-case handshake: %+v, %v", got, err)
	}
	for _, bad := range []pmkusum.Command{
		{Type: "data", Cmd: "read", MsgID: "1", Params: map[string]any{"A": 1}},
		{Type: "config", Cmd: "erase", MsgID: "1", Params: map[string]any{"A": 1}},
		{Type: "config", Cmd: "read", Params: map[string]any{"A": 1}},
		{Type: "config", Cmd: "read", MsgID: "1"},
		{Type: "config", Cmd: "write", MsgID: "1", Params: map[string]any{"msgid": 1}},
	} {
		if _, err := bad.Encode(ist); err == nil {
			t.Errorf("%+v was encoded", bad)
		}
	}
	if _, err := pmkusum.Interval("1", at, 16); err == nil {
		t.Error("a 16-minute interval was taken")
	}
	if _, err := pmkusum.Retrieve("1", at, 1, "2026-13-01", 3); err == nil {
		t.Error("a bad date was taken")
	}
}

// Every action builder makes a command that encodes.
func TestActions(t *testing.T) {
	at := time.Date(2026, 10, 11, 9, 30, 0, 0, ist)
	must := func(c pmkusum.Command, err error) pmkusum.Command {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	cmds := map[string]pmkusum.Command{
		"pump":     pmkusum.Pump("", "1", at, true),
		"restart":  pmkusum.Restart("2", at),
		"hb":       must(pmkusum.HeartbeatInterval("3", at, 5)),
		"clock":    pmkusum.SyncClock("4", at, at, ist),
		"gsm":      pmkusum.GSMSync("5", at),
		"apn":      must(pmkusum.APN("6", at, 2, "demo.apn", "u", "p")),
		"broker":   pmkusum.Broker("7", at, "rms.example.invalid", 8883, "d:860000000000001", "u", "p"),
		"firmware": pmkusum.Firmware("8", at, "ftp.example.invalid", 21, "u", "p"),
		"ai":       must(pmkusum.AnalogCalibration("9", at, 3, 4, 20)),
		"retrieve": must(pmkusum.Retrieve("10", at, 1, "2026-10-10", 5)),
	}
	want := map[string]map[string]any{
		"pump": {"DO1": 1}, "restart": {"RESTART": 1}, "hb": {"HEARTINTERVAL": 5},
		"clock": {"URTCDATE": 261011, "URTCTIME": 93000, "UPDATERTC": 1}, "gsm": {"GSMSYNC": 1},
		"apn":      {"APN2": "demo.apn", "USR2": "u", "PASS2": "p"},
		"broker":   {"URL": "rms.example.invalid", "PORT": 8883, "CID": "d:860000000000001", "USERNAME": "u", "PASSWORD": "p"},
		"firmware": {"FTPURL": "ftp.example.invalid", "FTPUSER": "u", "FTPPASS": "p", "FTPPORT": 21, "FTPDOWN": 1},
		"ai":       {"AI3ZERO": 4.0, "AI3SPAN": 20.0}, "retrieve": {"VD": 1, "DATE": 261010, "INDEX": 5, "LOAD": 1},
	}
	for name, c := range cmds {
		if _, err := c.Encode(ist); err != nil {
			t.Errorf("%s: %v", name, err)
		}
		if !reflect.DeepEqual(c.Params, want[name]) {
			t.Errorf("%s: params %v, want %v", name, c.Params, want[name])
		}
		for k := range c.Params {
			if !pmkusum.KnownKey(k) {
				t.Errorf("%s: %s is not an annexure 7 keyword", name, k)
			}
		}
	}
	if cmds["pump"].Type != pmkusum.TypeOnDemand || pmkusum.Pump(pmkusum.TypeConfig, "1", at, false).Type != pmkusum.TypeConfig {
		t.Error("pump message type")
	}
	if m := pmkusum.MaskSecrets(cmds["broker"].Params); m["PASSWORD"] != "***" || m["USERNAME"] != "u" {
		t.Errorf("masked %v", m)
	}
}

// The RMS's answer (Command.Answer) evaluated against the command:
// unknown keywords answered 0, a held output, an explicit refusal.
func TestAnswerAndEvaluate(t *testing.T) {
	at := time.Date(2026, 10, 11, 9, 30, 0, 0, ist)
	state := map[string]any{"DO1": 0, "UPDATEINTERVAL": 15}
	held := map[string]bool{"DO1": true}
	rms := func(k string, written any) (any, bool) {
		cur, ok := state[k]
		if !ok {
			return nil, false
		}
		if written == "" || held[k] {
			return cur, true
		}
		state[k] = written
		return written, true
	}
	ask := pmkusum.Write(pmkusum.TypeConfig, "50", at, map[string]any{"DO1": 1, "UPDATEINTERVAL": 10, "APNN1": 2})
	b, err := ask.Answer(rms, ist)
	if err != nil {
		t.Fatal(err)
	}
	ans, err := pmkusum.Decode(b, ist)
	if err != nil || ans.MsgID != "50" {
		t.Fatalf("answer %s: %v", b, err)
	}
	res := pmkusum.Evaluate(ask, ans)
	if res.OK() || !reflect.DeepEqual(res.Invalid, []string{"APNN1"}) || !reflect.DeepEqual(res.NotApplied, []string{"DO1"}) || res.Err() == nil {
		t.Errorf("result %+v", res)
	}
	read := pmkusum.Read(pmkusum.TypeOnDemand, "51", at, "UPDATEINTERVAL")
	b, _ = read.Answer(rms, ist)
	ans, _ = pmkusum.Decode(b, ist)
	if res := pmkusum.Evaluate(read, ans); !res.OK() || res.Values["UPDATEINTERVAL"] != 10.0 {
		t.Errorf("read result %+v", res)
	}
	no := false
	ans.Ack = &no
	if res := pmkusum.Evaluate(read, ans); !res.Refused || res.Err() == nil {
		t.Errorf("refusal %+v", res)
	}
}

func TestOTP(t *testing.T) {
	src := pmkusum.OTPSource{Secret: []byte("demo-secret-0123456789"), Interval: 30 * time.Minute, Digits: 8}
	at := time.Date(2026, 10, 11, 9, 40, 0, 0, ist)
	p, c := src.Pair("860000000000001", at)
	if len(p) != 8 || len(c) != 8 || p == c {
		t.Fatalf("pair %s %s", p, c)
	}
	if !src.Valid("860000000000001", c, at) || !src.Valid("860000000000001", p, at) {
		t.Error("the current or previous OTP was refused")
	}
	if src.Valid("860000000000001", c, at.Add(time.Hour)) || src.Valid("860000000000002", c, at) || src.Valid("860000000000001", "", at) {
		t.Error("a stale, foreign or empty OTP was taken")
	}
	if !src.WindowStart(at).Equal(time.Date(2026, 10, 11, 9, 30, 0, 0, ist)) {
		t.Errorf("window start %v", src.WindowStart(at))
	}
	b, err := src.Message("860000000000001", at).Encode(ist)
	if err != nil {
		t.Fatal(err)
	}
	o, err := pmkusum.DecodeOTP(b, ist)
	if err != nil || o.COTP != c || o.POTP != p || o.Validity != 30*time.Minute || !o.Time.Equal(at) {
		t.Errorf("otp message %s read as %+v, %v", b, o, err)
	}
	if _, err := pmkusum.DecodeOTP([]byte(`{"POTP":"1"}`), ist); err == nil {
		t.Error("an OTP message without COTP was taken")
	}
	if _, err := (pmkusum.OTPMessage{}).Encode(ist); err == nil {
		t.Error("an empty OTP message was encoded")
	}
}

func TestStorageDate(t *testing.T) {
	for in, want := range map[string]string{"261010": "2026-10-10", "2026-10-09": "2026-10-09"} {
		if got, err := pmkusum.ParseStorageDate(in); err != nil || got != want {
			t.Errorf("ParseStorageDate(%s) = %s, %v", in, got, err)
		}
	}
	if _, err := pmkusum.ParseStorageDate("18/05/2020"); err == nil {
		t.Error("an unknown DATE form was taken")
	}
	if s, _ := pmkusum.YYMMDD("2026-10-10"); s != "261010" {
		t.Errorf("YYMMDD = %s", s)
	}
}
