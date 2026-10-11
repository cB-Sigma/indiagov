// Copyright 2026 Abhinav Gautam
// SPDX-License-Identifier: Apache-2.0

package pmkusum

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Header is the section 6 header of an RMS message (the RMS's side).
type Header struct {
	VD   int
	Time time.Time
	// MaxIndex is the number of records a day holds (96 at 15 minutes),
	// Index this record's number in the day.
	MaxIndex, Index int
	Load            int
	// StInterval is the storage interval in minutes.
	StInterval int
	MsgID      string
	IMEI       string
	POTP, COTP string
	// Asset is the ASN_nn keyword ("ASN_11") and its value, for
	// equipment messages.
	Asset, AssetValue string
}

// Encode writes an RMS message: the header keywords, then keywords as
// given (equipment keywords with their instance digit, "PDKWH1"). TIMESTAMP
// is the RMS clock in loc; DATE is the YYMMDD number the annexure samples
// carry.
func Encode(h Header, loc *time.Location, keywords map[string]any) ([]byte, error) {
	if h.IMEI == "" {
		return nil, errors.New("IMEI is required")
	}
	if loc == nil {
		loc = time.UTC
	}
	t := h.Time.In(loc)
	date, _ := strconv.Atoi(t.Format("060102"))
	m := map[string]any{"VD": h.VD, "TIMESTAMP": t.Format("2006-01-02 15:04:05"), "MAXINDEX": h.MaxIndex, "INDEX": h.Index,
		"LOAD": h.Load, "STINTERVAL": h.StInterval, "MSGID": h.MsgID, "DATE": date, "IMEI": h.IMEI, "POTP": h.POTP, "COTP": h.COTP}
	if h.Asset != "" {
		if !strings.HasPrefix(strings.ToUpper(h.Asset), "ASN_") {
			return nil, fmt.Errorf("asset keyword %q is not ASN_nn", h.Asset)
		}
		m[strings.ToUpper(h.Asset)] = h.AssetValue
	}
	for k, v := range keywords {
		if _, dup := m[upper(k)]; dup {
			return nil, fmt.Errorf("%s is a header keyword", k)
		}
		m[k] = v
	}
	return json.Marshal(m)
}

// Keyword is the keyword of a value name of a group's table with the
// instance digit appended (GroupPump, "outputPower", 1 → "POPKW1"); the
// RMS heartbeat's keywords have no digit.
func Keyword(g Group, name string, instance int) (string, bool) {
	for k, p := range tables[g] {
		if p.Name != name {
			continue
		}
		if g == GroupRMS {
			return k, true
		}
		if instance < 1 || instance > 9 {
			return "", false
		}
		return k + strconv.Itoa(instance), true
	}
	return "", false
}

// AssetKeyword is the ASN_nn keyword of a group's instance (GroupPump, 1 →
// "ASN_11"; GroupRMS → "ASN_00").
func AssetKeyword(g Group, instance int) (string, bool) {
	base := map[Group]int{GroupRMS: 0, GroupDAQ: 0, GroupPump: 10, GroupMeter: 20, GroupInverter: 30, GroupSJB: 40}
	b, ok := base[g]
	if !ok {
		return "", false
	}
	if g == GroupRMS {
		return "ASN_00", true
	}
	if instance < 1 || instance > 9 {
		return "", false
	}
	return fmt.Sprintf("ASN_%02d", b+instance), true
}

// Params lists a group's table: keyword (without instance digit) to Param.
func Params(g Group) map[string]Param {
	out := map[string]Param{}
	for k, p := range tables[g] {
		out[k] = p
	}
	return out
}

func upper(k string) string { return strings.ToUpper(strings.TrimSpace(k)) }
