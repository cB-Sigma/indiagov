// Copyright 2026 Abhinav Gautam
// SPDX-License-Identifier: Apache-2.0

package jjm

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// NewPayload builds a device message: dt is t in loc without an offset, as
// the report writes it, and each value is sent as text the way the samples
// do (a nil value is "", no value). values are by parameter code.
func NewPayload(gid, did, dty string, t time.Time, loc *time.Location, values map[string]any) (Payload, error) {
	if !GIDPattern.MatchString(gid) || !GIDPattern.MatchString(did) {
		return Payload{}, errors.New("gid and did must be 1 to 64 letters, digits, '-', '.', '_' or '~'")
	}
	if loc == nil {
		loc = time.UTC
	}
	p := map[string]any{}
	for code, v := range values {
		switch x := v.(type) {
		case nil:
			p[code] = ""
		case string:
			p[code] = x
		case float64:
			p[code] = strconv.FormatFloat(x, 'f', -1, 64)
		case float32:
			p[code] = strconv.FormatFloat(float64(x), 'f', -1, 32)
		case int:
			p[code] = strconv.Itoa(x)
		case int64:
			p[code] = strconv.FormatInt(x, 10)
		case bool:
			p[code] = map[bool]string{true: "1", false: "0"}[x]
		default:
			return Payload{}, fmt.Errorf("%s: a value must be text, a number, a bool or nil", code)
		}
	}
	return Payload{GID: gid, DID: did, DTY: strings.ToLower(dty), DT: t.In(loc).Format("2006-01-02T15:04:05"), P: p}, nil
}

// JSON is the message as sent.
func (p Payload) JSON() ([]byte, error) { return json.Marshal(p) }

// ErrorCode values a device sends in e (annexure 9(c)(i); section 5(f)
// numbers the flow meter's differently, see Decoder.FlowErrorCodes).
const (
	ErrorNone      = "00"
	ErrorTamper    = "01"
	ErrorOther     = "02"
	ErrorEndOfDay  = "11"
	ErrorLeakage5f = "01" // section 5(f) flow meter numbering
	ErrorTamper5f  = "02"
	ErrorOther5f   = "03"
)

// Metadata is what the report's annexure 9(c)(ii) asks to record for each
// device when it is deployed, in our field names. The report gives the
// items, not a message, so the JSON names are ours.
type Metadata struct {
	GID                 string   `json:"gid"`
	DID                 string   `json:"did"`
	DTY                 string   `json:"dty"`
	Name                string   `json:"name,omitempty"`
	Latitude            *float64 `json:"latitude,omitempty"`
	Longitude           *float64 `json:"longitude,omitempty"`
	Altitude            *float64 `json:"altitude,omitempty"`
	LocationDescription string   `json:"locationDescription,omitempty"`
	Dimensions          string   `json:"dimensions,omitempty"`
	Model               string   `json:"model,omitempty"`
	SerialNumber        string   `json:"serialNumber,omitempty"`
	Power               string   `json:"power,omitempty"` // mains, solar or battery
	Communication       string   `json:"communication,omitempty"`
	InstalledOn         string   `json:"installedOn,omitempty"`
	InstalledBy         string   `json:"installedBy,omitempty"`
}

// Check reports what is missing or malformed.
func (m Metadata) Check() error {
	if !GIDPattern.MatchString(m.GID) || !GIDPattern.MatchString(m.DID) {
		return errors.New("gid and did must be 1 to 64 letters, digits, '-', '.', '_' or '~'")
	}
	if _, ok := DeviceTypes[strings.ToLower(m.DTY)]; !ok {
		return fmt.Errorf("dty %q is not a device type", m.DTY)
	}
	if m.Latitude != nil && (*m.Latitude < -90 || *m.Latitude > 90) {
		return errors.New("latitude must be -90 to 90")
	}
	if m.Longitude != nil && (*m.Longitude < -180 || *m.Longitude > 180) {
		return errors.New("longitude must be -180 to 180")
	}
	switch m.Power {
	case "", "mains", "solar", "battery":
	default:
		return fmt.Errorf("power %q must be mains, solar or battery", m.Power)
	}
	return nil
}

// Interval is the transmission interval the report expects of a device
// type for its power source ("battery" or anything else for mains or
// solar with battery backup); 0 when the report gives none.
func Interval(dty, power string) time.Duration {
	dt, ok := DeviceTypes[strings.ToLower(dty)]
	if !ok {
		return 0
	}
	if power == "battery" && dt.Battery > 0 {
		return dt.Battery
	}
	return dt.Mains
}
