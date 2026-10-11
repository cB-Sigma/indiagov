package pmkusum

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Message is one decoded RMS message of the profile.
type Message struct {
	// Raw holds every keyword as sent, upper-cased keys.
	Raw map[string]any
	// Header keywords (section 6).
	IMEI    string
	VD      *int
	MsgID   string
	Command string // COMMAND or cmd
	Type    string // "type" of annexure 7 messages
	Time    time.Time
	HasTime bool
	// TimeErr is why TIMESTAMP could not be read (the message is kept
	// without HasTime).
	TimeErr     error
	StorageDate string // DATE as YYYY-MM-DD
	Index       *int
	MaxIndex    *int
	Load        *int
	StInterval  *int
	POTP, COTP  string
	// Equipment.
	Group    Group
	Instance int
	Asset    string // ASN_nn value
	// Values by our name (numbers as float64, text as string), per
	// instance for tables with an instance digit.
	Values map[string]any
	// Other holds keywords no table knows, as sent.
	Other map[string]any
	// Answer marks a command answer of a profile whose answers do not
	// come on the ondemand or config topic; Ack is its explicit outcome
	// (nil: judged from the echoed keywords) and AckText the RMS's words.
	Answer  bool
	Ack     *bool
	AckText string
}

// Decode parses an RMS JSON message. loc is the RMS clock's time zone
// (section 6 gives the RTC timestamp without one).
func Decode(payload []byte, loc *time.Location) (*Message, error) {
	d := json.NewDecoder(bytes.NewReader(payload))
	d.UseNumber()
	var raw map[string]any
	if err := d.Decode(&raw); err != nil {
		return nil, fmt.Errorf("the message is not a JSON object: %w", err)
	}
	m := &Message{Raw: map[string]any{}, Values: map[string]any{}, Other: map[string]any{}}
	for k, v := range raw {
		m.Raw[strings.ToUpper(strings.TrimSpace(k))] = v
	}
	m.IMEI = text(m.Raw["IMEI"])
	m.MsgID = text(m.Raw["MSGID"])
	m.Command = strings.ToLower(text(m.Raw["COMMAND"]))
	if c := text(m.Raw["CMD"]); c != "" {
		m.Command = strings.ToLower(c)
	}
	m.Type = strings.ToLower(text(m.Raw["TYPE"]))
	m.POTP, m.COTP = text(m.Raw["POTP"]), text(m.Raw["COTP"])
	m.VD, m.Index, m.MaxIndex, m.Load, m.StInterval = intp(m.Raw["VD"]), intp(m.Raw["INDEX"]), intp(m.Raw["MAXINDEX"]), intp(m.Raw["LOAD"]), intp(m.Raw["STINTERVAL"])
	if ts := text(m.Raw["TIMESTAMP"]); ts != "" {
		if t, err := ParseTimestamp(ts, loc); err != nil {
			m.TimeErr = err
		} else {
			m.Time, m.HasTime = t, true
		}
	}
	if ds := text(m.Raw["DATE"]); ds != "" {
		if s, err := ParseStorageDate(ds); err == nil {
			m.StorageDate = s
		}
	}
	// The equipment: the ASN_nn keyword names the group and instance.
	for k, v := range m.Raw {
		if n, ok := strings.CutPrefix(k, "ASN_"); ok {
			i, err := strconv.Atoi(n)
			if err != nil {
				continue
			}
			if g, inst, ok := groupOfASN(i); ok {
				m.Group, m.Instance, m.Asset = g, inst, text(v)
			}
		}
	}
	if m.Group == "" {
		m.Group, m.Instance = guessGroup(m.Raw)
	}
	for k, v := range m.Raw {
		if headerKeys[k] || strings.HasPrefix(k, "ASN_") || k == "TYPE" || k == "CMD" {
			continue
		}
		if ch, name, p, ok := uspcKey(k); ok && (m.Group == GroupPump || m.Group == "") {
			chans, _ := m.Values["channels"].(map[string]any)
			if chans == nil {
				chans = map[string]any{}
				m.Values["channels"] = chans
			}
			c, _ := chans[itoa(ch)].(map[string]any)
			if c == nil {
				c = map[string]any{}
				chans[itoa(ch)] = c
			}
			c[name] = value(v, p.Text)
			continue
		}
		name, p, ok := m.param(k)
		if !ok {
			m.Other[k] = plain(v)
			continue
		}
		m.Values[name] = value(v, p.Text)
	}
	return m, nil
}

// param finds a keyword in the message's group table.
func (m *Message) param(k string) (string, Param, bool) {
	if m.Group == GroupRMS || m.Group == "" {
		if p, ok := lookup(GroupRMS, k); ok {
			return p.Name, p, true
		}
		if m.Group == GroupRMS {
			return "", Param{}, false
		}
	}
	base, inst := splitInstance(k)
	if inst == 0 {
		return "", Param{}, false
	}
	p, ok := lookup(m.Group, base)
	if !ok || (m.Instance != 0 && inst != m.Instance) {
		return "", Param{}, false
	}
	if m.Instance == 0 {
		m.Instance = inst
	}
	return p.Name, p, true
}

// guessGroup names the group whose table knows most of the keywords, for
// messages without an ASN_nn keyword (the annexure 5 heartbeat and the
// annexure 6 DAQ sample have none).
func guessGroup(raw map[string]any) (Group, int) {
	best, bestN, inst := Group(""), 0, 0
	groups := []Group{GroupRMS, GroupPump, GroupMeter, GroupInverter, GroupSJB, GroupDAQ}
	for _, g := range groups {
		n, in := 0, 0
		for k := range raw {
			if headerKeys[k] {
				continue
			}
			if g == GroupRMS {
				if _, ok := lookup(g, k); ok {
					n++
				}
				continue
			}
			base, i := splitInstance(k)
			if g == GroupPump {
				if _, _, _, ok := uspcKey(k); ok {
					n++
					continue
				}
			}
			if _, ok := lookup(g, base); ok && i > 0 {
				n++
				in = i
			}
		}
		if n > bestN {
			best, bestN, inst = g, n, in
		}
	}
	return best, inst
}

// timestampForms are the TIMESTAMP layouts read besides the section 6
// form: ISO 8601 with a T, slashes, day first and without seconds, which
// RMS firmware is seen to send.
var timestampForms = []string{"2006-01-02 15:04:05", "2006-01-02T15:04:05", "2006/01/02 15:04:05",
	"02-01-2006 15:04:05", "02/01/2006 15:04:05", "2006-01-02 15:04"}

// ParseTimestamp reads the section 6 RTC timestamp "YYYY-MM-DD HH:mm:SS"
// (in loc), the variants of timestampForms, and RFC 3339 with an offset.
func ParseTimestamp(s string, loc *time.Location) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, f := range timestampForms {
		if t, err := time.ParseInLocation(f, s, loc); err == nil {
			return t, nil
		}
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("TIMESTAMP %q is not YYYY-MM-DD HH:mm:SS", s)
}

// ParseStorageDate reads DATE in either printed form: YYYY-MM-DD (section
// 6) or the number YYMMDD (the annexure samples), returning YYYY-MM-DD.
func ParseStorageDate(s string) (string, error) {
	s = strings.TrimSpace(s)
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.Format("2006-01-02"), nil
	}
	if len(s) == 6 {
		if t, err := time.Parse("060102", s); err == nil {
			return t.Format("2006-01-02"), nil
		}
	}
	return "", fmt.Errorf("DATE %q is neither YYYY-MM-DD nor YYMMDD", s)
}

// YYMMDD formats a date the way the annexure samples carry DATE.
func YYMMDD(date string) (string, error) {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return "", errors.New("the date must be YYYY-MM-DD")
	}
	return t.Format("060102"), nil
}

// text returns a keyword's value as text.
func text(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(x)
	case json.Number:
		return x.String()
	default:
		b, _ := json.Marshal(x)
		return string(b)
	}
}

func intp(v any) *int {
	s := text(v)
	if s == "" {
		return nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	i := int(f)
	return &i
}

// value turns a keyword's value into a number when it is one (the samples
// send numbers both bare and quoted: "PDC1V1":"550.00", "IR1":20.58).
func value(v any, asText bool) any {
	if asText {
		return text(v)
	}
	switch x := v.(type) {
	case json.Number:
		f, _ := x.Float64()
		return f
	case string:
		s := strings.TrimSpace(x)
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f
		}
		return s
	}
	return plain(v)
}

func plain(v any) any {
	switch x := v.(type) {
	case json.Number:
		f, _ := x.Float64()
		return f
	case map[string]any:
		out := map[string]any{}
		for k, e := range x {
			out[k] = plain(e)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = plain(e)
		}
		return out
	}
	return v
}

func num(v any) (float64, bool) {
	f, ok := v.(float64)
	return f, ok
}

// Content is what the IPE writes for an equipment message.
func (m *Message) Content() map[string]any {
	out := map[string]any{}
	for k, v := range m.Values {
		out[k] = v
	}
	if m.Group == GroupPump {
		if code, ok := num(m.Values["runStatus"]); ok {
			if s, ok := RunStatus[int(code)]; ok {
				out["runStatusText"] = s
			}
		}
		if code, ok := num(m.Values["controlMode"]); ok {
			if s, ok := ControlMode[int(code)]; ok {
				out["controlModeText"] = s
			}
		}
		// PV power is required by the MNRE USPC and RMS specifications but
		// has no keyword in the profile: derived from the DC input.
		v, okV := num(m.Values["dcVoltage"])
		i, okI := num(m.Values["dcCurrent"])
		if okV && okI {
			out["pvPower"] = round3(v * i / 1000)
		}
	}
	if m.Asset != "" {
		out["asset"] = m.Asset
	}
	if m.Instance > 0 {
		out["instance"] = m.Instance
	}
	if m.VD != nil {
		out["vd"] = *m.VD
	}
	if m.Index != nil {
		out["index"] = *m.Index
	}
	if m.MaxIndex != nil {
		out["maxIndex"] = *m.MaxIndex
	}
	if m.Load != nil {
		out["load"] = *m.Load
	}
	if m.StInterval != nil {
		out["storeInterval"] = *m.StInterval
	}
	if m.StorageDate != "" {
		out["storageDate"] = m.StorageDate
	}
	if len(m.Other) > 0 {
		out["other"] = m.Other
	}
	return out
}

func round3(f float64) float64 {
	return float64(int64(f*1000+0.5)) / 1000
}

// Container is the device container an equipment message goes to.
func (m *Message) Container() string {
	switch m.Group {
	case GroupRMS:
		return "heartbeat"
	case "":
		return "unclassified"
	}
	inst := m.Instance
	if inst == 0 {
		inst = 1
	}
	return string(m.Group) + "-" + itoa(inst)
}

// sortedKeys returns a map's keys in order.
func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
