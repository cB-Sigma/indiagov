package jjm

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// The device-to-cloud data format of the Jal Jeevan Mission Technical/
// Expert Committee report "Measurement and monitoring of water service
// delivery in rural areas" (NJJM, DDWS, March 2021): section 5(f) and
// annexure 9(c)(i):
//
//	{"gid": "MH_D02_B03_V04", "did": "fm01", "dty": "fm", "dt": "2026-09-20T06:00:00",
//	 "p": {"t": "48250.5", "f": "212.4", "b": "87", "e": "00", "p5": "", "p6": ""}}
//
// gid is the gateway or location ID (state, district, block, revenue
// village and scheme), did the device ID, dty the device type, dt the time
// stamp and p the parameters, sent as text; "" is no value.

// Param is one parameter code of p.
type Param struct {
	Name string
	Unit string
	// Min and Max are the range a valid value lies in (NaN: none).
	Min, Max float64
	// Text keeps the value as text.
	Text bool
	// Ours marks a code the report names as data to capture without a
	// code (F-487).
	Ours bool
	// API marks a code of ours for a value the ejalshakti push API names.
	API bool
	// Kind names the unit table a sent value may be converted from
	// (units.go); empty: only Unit.
	Kind string
}

var inf = math.Inf(1)

// Params are the section 5(f) (ii) and annexure 9(c)(i) parameter codes.
// Ranges come from the report's annexure 9(b) reference hardware (chlorine
// analyser 0 to 10 mg/L, pressure transmitter 0.1 to 60 bar) and the
// quantities' nature (battery 0 to 100, level percent 0 to 100, pH 0 to 14).
var Params = map[string]Param{
	"t":   {Name: "quantity", Unit: "L", Min: 0, Max: inf, Kind: "volume"},
	"f":   {Name: "flowRate", Unit: "L/min", Min: 0, Max: inf, Kind: "flow"},
	"b":   {Name: "battery", Unit: "%", Min: 0, Max: 100},
	"g":   {Name: "groundWaterDepth", Unit: "m", Min: 0, Max: inf, Kind: "length"},
	"l":   {Name: "reservoirLevel", Min: 0, Max: inf},
	"lp":  {Name: "reservoirLevelPercent", Unit: "%", Min: 0, Max: 100},
	"lm":  {Name: "reservoirLevelMetres", Unit: "m", Min: 0, Max: inf, Kind: "length"},
	"pr":  {Name: "pressure", Unit: "bar", Min: 0, Max: 60, Kind: "pressure"},
	"c":   {Name: "chlorine", Unit: "mg/L", Min: 0, Max: 10, Kind: "concentration"},
	"tb":  {Name: "turbidity", Unit: "NTU", Min: 0, Max: inf},
	"ph":  {Name: "ph", Unit: "pH", Min: 0, Max: 14},
	"cd":  {Name: "conductivity", Unit: "uS/cm", Min: 0, Max: inf, Kind: "conductivity"},
	"a":   {Name: "current", Unit: "A", Min: 0, Max: inf},
	"v":   {Name: "voltage", Unit: "V", Min: 0, Max: inf},
	"s":   {Name: "pumpStatus", Min: 0, Max: 1},
	"vb":  {Name: "vibration", Unit: "Hz", Min: 0, Max: inf},
	"kwh": {Name: "energy", Unit: "kWh", Min: 0, Max: inf, Ours: true},
	"pf":  {Name: "powerFactor", Min: -1, Max: 1, Ours: true},
	"e":   {Name: "errorCode", Text: true},
	// The eJalShakti IoT push API adds the village's daily supply and LPCD
	// (FlowMeter) and, in its "IoT Web API Description" overview only, the
	// water quality parameters below; it states no units (F-2705), so the
	// units are the usual ones of each quantity and Decoder.Units converts
	// others.
	"ws":   {Name: "dailyWaterSupply", Unit: "L", Min: 0, Max: inf, Kind: "volume", API: true},
	"lpcd": {Name: "lpcd", Unit: "L/(cap.d)", Min: 0, Max: inf, API: true},
	"no3":  {Name: "nitrate", Unit: "mg/L", Min: 0, Max: inf, Kind: "concentration", API: true},
	"fl":   {Name: "fluoride", Unit: "mg/L", Min: 0, Max: inf, Kind: "concentration", API: true},
	"fe":   {Name: "iron", Unit: "mg/L", Min: 0, Max: inf, Kind: "concentration", API: true},
	"sal":  {Name: "salinity", Min: 0, Max: inf, API: true},
	"as":   {Name: "arsenic", Unit: "mg/L", Min: 0, Max: inf, Kind: "concentration", API: true},
	"tds":  {Name: "tds", Unit: "mg/L", Min: 0, Max: inf, Kind: "concentration", API: true},
}

// DeviceTypes are the annexure 9(c)(i) device type codes with the
// parameters each sample carries and the report's collection and
// transmission intervals (section 5(f) (ii); battery, mains or solar).
type DeviceType struct {
	Name    string
	Params  []string
	Battery time.Duration // expected transmission interval on battery (0: not given)
	Mains   time.Duration // on mains or solar with battery backup
	// URLName is the central API method name ({URLName}DataInsert); only
	// Flowmeter is printed, the others are ours (F-490).
	URLName string
}

// DeviceTypes by dty code.
var DeviceTypes = map[string]DeviceType{
	"fm":  {Name: "flow meter", Params: []string{"t", "f", "b", "e"}, Battery: 12 * time.Hour, Mains: 15 * time.Minute, URLName: "Flowmeter"},
	"gl":  {Name: "ground water level sensor", Params: []string{"g", "b", "e"}, Battery: 24 * time.Hour, Mains: time.Hour, URLName: "GroundWaterLevel"},
	"sl":  {Name: "service reservoir level sensor", Params: []string{"lp", "lm", "b", "e"}, Battery: 24 * time.Hour, Mains: 15 * time.Minute, URLName: "ServiceReservoirLevel"},
	"ca":  {Name: "chlorine analyser", Params: []string{"c", "b", "e"}, Mains: 15 * time.Minute, URLName: "ChlorineAnalyser"},
	"tbs": {Name: "turbidity sensor", Params: []string{"tb", "b", "e"}, Mains: 15 * time.Minute, URLName: "Turbidity"},
	"ph":  {Name: "pH analyser", Params: []string{"ph", "b", "e"}, Mains: 15 * time.Minute, URLName: "PHAnalyser"},
	"ps":  {Name: "pressure sensor", Params: []string{"pr", "b", "e"}, Mains: 5 * time.Minute, URLName: "Pressure"},
	"pc":  {Name: "pump controller", Params: []string{"a", "v", "s", "vb", "e"}, Mains: time.Hour, URLName: "PumpController"},
	// Water quality analysers of the eJalShakti overview's unlisted APIs
	// (codes ours, F-2705); no transmission interval is given for them.
	"no3": {Name: "nitrate analyser", Params: []string{"no3", "b", "e"}, URLName: "Nitrate"},
	"fl":  {Name: "fluoride analyser", Params: []string{"fl", "b", "e"}, URLName: "Fluoride"},
	"fe":  {Name: "iron analyser", Params: []string{"fe", "b", "e"}, URLName: "Iron"},
	"sal": {Name: "salinity sensor", Params: []string{"sal", "b", "e"}, URLName: "Salinity"},
	"as":  {Name: "arsenic analyser", Params: []string{"as", "b", "e"}, URLName: "Arsenic"},
	"tds": {Name: "TDS sensor", Params: []string{"tds", "b", "e"}, URLName: "TDS"},
}

// Error code tables. Annexure 9(c)(i): 00 no error, 01 tamper, 02 other
// error, and for flow meters and ground water level sensors 11 active
// status at end of day. Section 5(f) gives the flow meter 01 leakage, 02
// tamper, 03 other error (F-484).
var (
	errorCodesAnnex    = map[string]string{"00": "", "01": "tamper", "02": "other", "11": "end-of-day"}
	errorCodesSection5 = map[string]string{"00": "", "01": "leakage", "02": "tamper", "03": "other", "11": "end-of-day"}
)

// Payload is one device-to-cloud message.
type Payload struct {
	GID string            `json:"gid"`
	DID string            `json:"did"`
	DTY string            `json:"dty"`
	DT  string            `json:"dt"`
	P   map[string]any    `json:"p"`
	raw map[string]string // p as text
	// VillageID and GatewayID are the ejalshakti portal's IDs of the
	// sender, when the message came in that API's form.
	VillageID string `json:"-"`
	GatewayID string `json:"-"`
}

// Reading is a decoded, validated message.
type Reading struct {
	GID, DID, DTY string
	Time          time.Time
	HasTime       bool
	// Values by our parameter name: float64, or nil for "" (no value).
	Values map[string]any
	// Codes maps our name back to the report's code.
	Codes map[string]string
	// ErrorCode is e as sent; Error its meaning ("" none).
	ErrorCode, Error string
	// Flags are data quality flags: error:<meaning>, range:<code>,
	// invalid:<code>, future, no-time, unknown-code:<code>.
	Flags []string
	// Unknown holds parameter codes no table knows, as sent.
	Unknown map[string]string
	// Raw keeps each converted value as sent with its unit, by our name.
	Raw    map[string]RawValue
	Source Payload
	// TimeErr is why dt could not be read (the reading then has the
	// receipt time and the no-time flag); Unreadable are parameter codes
	// whose value was neither text nor a number (left out, flagged
	// invalid:<code>).
	TimeErr    string
	Unreadable []string
	// VillageID and GatewayID are the ejalshakti portal's IDs (Payload).
	VillageID, GatewayID string
}

// Location is a gid split into its parts (section 5(f): State, District,
// Block, Revenue Village and Scheme; the samples show four parts).
type Location struct {
	State, District, Block, Village, Scheme string
}

var GIDPattern = regexp.MustCompile(`^[A-Za-z0-9._~-]{1,64}$`)

// ParseGID splits a gid such as MH_D02_B03_V04 (a fifth part is the
// scheme). ok is false when it does not have four or five parts.
func ParseGID(gid string) (Location, bool) {
	p := strings.Split(gid, "_")
	if len(p) < 4 || len(p) > 5 {
		return Location{}, false
	}
	for _, x := range p {
		if x == "" {
			return Location{}, false
		}
	}
	l := Location{State: p[0], District: p[1], Block: p[2], Village: p[3]}
	if len(p) == 5 {
		l.Scheme = p[4]
	}
	return l, true
}

// Decoder decodes and checks messages.
type Decoder struct {
	Loc *time.Location
	// FlowErrorCodes is "annex" (default) or "section5".
	FlowErrorCodes string
	// GroundWaterUnit is the unit ground water depth is sent in: "ft"
	// (annexure 9(c), default) or "m" (section 5(f)); it is converted to m.
	GroundWaterUnit string
	// Units are the units values are sent in, by parameter code, where they
	// differ from the report's (converted to the report's unit).
	Units map[string]string
	// FutureSkew is how far ahead of the server clock dt may be.
	FutureSkew time.Duration
	Now        func() time.Time
}

// Decode parses and validates one message.
func (d Decoder) Decode(b []byte) (*Reading, error) {
	var p Payload
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("the message is not a JSON object of gid, did, dty, dt and p: %w", err)
	}
	return d.Check(p)
}

// Check validates a parsed message.
func (d Decoder) Check(p Payload) (*Reading, error) {
	if p.GID == "" || !GIDPattern.MatchString(p.GID) {
		return nil, errors.New("gid is missing or has characters other than letters, digits, '-', '.', '_' or '~'")
	}
	if p.DID == "" || !GIDPattern.MatchString(p.DID) {
		return nil, errors.New("did is missing or has characters other than letters, digits, '-', '.', '_' or '~'")
	}

	r := &Reading{GID: p.GID, DID: p.DID, DTY: strings.ToLower(p.DTY), Values: map[string]any{}, Codes: map[string]string{},
		Unknown: map[string]string{}, Raw: map[string]RawValue{}, Source: p, VillageID: p.VillageID, GatewayID: p.GatewayID}
	p.raw = map[string]string{}
	now := time.Now
	if d.Now != nil {
		now = d.Now
	}
	if p.DT == "" {
		r.Time = now()
		r.Flags = append(r.Flags, "no-time")
	} else {
		t, err := parseDT(p.DT, d.Loc)
		if err != nil {
			// Kept with the receipt time: the values are still the device's.
			r.Time, r.TimeErr = now(), err.Error()
			r.Flags = append(r.Flags, "no-time")
		} else {
			r.Time, r.HasTime = t, true
		}
		skew := d.FutureSkew
		if skew == 0 {
			skew = 5 * time.Minute
		}
		if r.HasTime && r.Time.After(now().Add(skew)) {
			r.Flags = append(r.Flags, "future")
		}
	}
	for _, code := range sortedKeys(p.P) {
		v := p.P[code]
		s, ok := asText(v)
		if !ok {
			r.Unreadable = append(r.Unreadable, code)
			r.Flags = append(r.Flags, "invalid:"+strings.ToLower(code))
			continue
		}
		k := strings.ToLower(code)
		if k == "e" {
			r.ErrorCode = s
			continue
		}
		prm, known := Params[k]
		if !known {
			if s != "" && !placeholder(k) {
				r.Unknown[code] = s
			}
			continue
		}
		r.Codes[prm.Name] = k
		if s == "" {
			r.Values[prm.Name] = nil
			continue
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			r.Flags = append(r.Flags, "invalid:"+k)
			continue
		}
		if from := d.unitOf(k); from != prm.Unit {
			r.Raw[prm.Name] = RawValue{V: f, U: from}
			f = convert(prm, from, f)
		}
		r.Values[prm.Name] = f
		if f < prm.Min || f > prm.Max {
			r.Flags = append(r.Flags, "range:"+k)
		}
	}
	if r.ErrorCode != "" {
		table := errorCodesAnnex
		if r.DTY == "fm" && d.FlowErrorCodes == "section5" {
			table = errorCodesSection5
		}
		meaning, ok := table[r.ErrorCode]
		// Code 11 is printed for flow meters and ground water level sensors
		// only; elsewhere it is not a known code.
		if ok && r.ErrorCode == "11" && r.DTY != "fm" && r.DTY != "gl" && r.DTY != "sl" {
			ok = false
		}
		switch {
		case !ok:
			r.Error = "unknown"
			r.Flags = append(r.Flags, "unknown-code:"+r.ErrorCode)
		case meaning == "end-of-day":
			r.Error = ""
		default:
			r.Error = meaning
			if meaning != "" {
				r.Flags = append(r.Flags, "error:"+meaning)
			}
		}
	}
	return r, nil
}

// placeholder reports the unused slots of the samples (p4, p5, p6).
func placeholder(k string) bool {
	return len(k) == 2 && k[0] == 'p' && k[1] >= '1' && k[1] <= '9'
}

// Quality is the reading's quality string ("good" or its flags).
func (r *Reading) Quality() string {
	if len(r.Flags) == 0 {
		return "good"
	}
	return strings.Join(r.Flags, ",")
}

// EndOfDay reports the code 11 daily status message.
func (r *Reading) EndOfDay() bool { return r.ErrorCode == "11" && r.Error == "" }

func parseDT(s string, loc *time.Location) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if loc == nil {
		loc = time.UTC
	}
	if t, err := time.ParseInLocation("2006-01-02T15:04:05", s, loc); err == nil {
		return t, nil
	}
	// Forms devices are seen to send besides the report's.
	for _, f := range []string{"2006-01-02 15:04:05", "2006/01/02 15:04:05", "02-01-2006 15:04:05", "02/01/2006 15:04:05", "2006-01-02T15:04",
		"2006-01-02T15:04:05.999999999", "2006-01-02"} {
		if t, err := time.ParseInLocation(f, s, loc); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("dt %q is not a date and time such as 2026-09-20T06:00:00", s)
}

func asText(v any) (string, bool) {
	switch x := v.(type) {
	case nil:
		return "", true
	case string:
		return strings.TrimSpace(x), true
	case json.Number:
		return x.String(), true
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64), true
	case bool:
		if x {
			return "1", true
		}
		return "0", true
	}
	return "", false
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Content is the telemetry container's entry for a reading.
func (r *Reading) Content() map[string]any {
	out := map[string]any{"gid": r.GID, "did": r.DID, "dty": r.DTY, "values": r.Values, "quality": r.Quality()}
	if r.ErrorCode != "" {
		out["errorCode"] = r.ErrorCode
		if r.Error != "" {
			out["error"] = r.Error
		}
	}
	if r.EndOfDay() {
		out["endOfDay"] = true
	}
	if len(r.Unknown) > 0 {
		out["unknown"] = r.Unknown
	}
	if len(r.Raw) > 0 {
		out["raw"] = r.Raw
	}
	return out
}

// Wire is the report's message form of a reading (text values), for the
// central API.
func (r *Reading) Wire(loc *time.Location) map[string]any {
	p := map[string]any{}
	for k, v := range r.Source.P {
		s, _ := asText(v)
		p[k] = s
	}
	dt := r.Source.DT
	if dt == "" {
		if loc == nil {
			loc = time.UTC
		}
		dt = r.Time.In(loc).Format("2006-01-02T15:04:05")
	}
	return map[string]any{"gid": r.GID, "did": r.DID, "dty": r.Source.DTY, "dt": dt, "p": p}
}
