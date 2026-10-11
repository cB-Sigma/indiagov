package jjm

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

// The eJalShakti IoT push API (DDWS/NIC: the help pages at
// https://ejalshakti.gov.in/IoT_API/Help, the "IoT Data Integration -
// Operational Manual" of 18 February 2026 and its 2021 edition, and the
// "IoT Web API Description") is how agencies put IoT readings on the
// national JJM dashboard. Each API is a POST to {base}/{API} with a JSON
// (or DataContract XML) body naming the portal's VillageId and GatewayId,
// and the key the portal issued for that API in the Authorization header,
// as it is. FlowMeter takes one value per village and day (WaterSupply and
// LPCD, with Date); the others take readings with their TimeStamp, in a
// collection (the manuals' samples) or one at the top level beside it (the
// help pages show both). The answer is {"Status", "Message"}; the manual's
// screenshots show Status true both on success (Message "sucess") and for
// a key that does not match (F-2705).
//
// This file holds the API table, the readers of the request bodies (JSON
// and DataContract XML) and the envelope of a queued push. A server of
// the API (an edge gateway or a state server standing in for the portal)
// and a client that pushes to the portal are built on them; SigmaM2M's
// JJM adapter is both.

// EjalshaktiURL is the portal's API base.
const EjalshaktiURL = "https://ejalshakti.gov.in/IoT_API/API"

// EjalshaktiSuccess is the portal's success message, spelt as it sends it.
const EjalshaktiSuccess = "sucess"

// PortalAPI is one ejalshakti API.
type PortalAPI struct {
	Name  string // the URL's last segment
	DTY   string // our device type
	Daily bool   // one value per village and day (Date)
	// Value is the value member and Code our parameter code; List is the
	// collection member of the series APIs.
	Value, Code, List string
	// Value2/Code2: FlowMeter's second value (LPCD).
	Value2, Code2 string
	// XMLRoot is the DataContract root element and XMLItem the element of
	// one collection entry (the help pages' XML samples).
	XMLRoot, XMLItem string
	// Retired marks an API of the 2021 manual that the 2026 manual and
	// the help pages no longer list (GroundWater, Pressure): read, and
	// pushed only when the client allows it.
	Retired bool
	// Unlisted marks an API named only in the "IoT Web API Description"
	// overview, not on the help pages or in the 2026 manual (checked
	// 2026-10-11): read, and pushed only when the client allows it.
	Unlisted bool
}

// Current reports an API the help pages list.
func (a PortalAPI) Current() bool { return !a.Retired && !a.Unlisted }

var PortalAPIs = []PortalAPI{
	{Name: "FlowMeter", DTY: "fm", Daily: true, Value: "WaterSupply", Code: "ws", Value2: "LPCD", Code2: "lpcd", XMLRoot: "Flowmeterinput"},
	{Name: "GroundWater", DTY: "gl", Daily: true, Value: "Depth", Code: "g", XMLRoot: "GroundWaterinput", Retired: true},
	{Name: "Chlorine", DTY: "ca", Value: "ResidualCL", Code: "c", List: "ChlorineData", XMLRoot: "Chlorineinput", XMLItem: "ChlorineData"},
	{Name: "PHLevel", DTY: "ph", Value: "PH_Level", Code: "ph", List: "PhLevelData", XMLRoot: "PhLevelinput", XMLItem: "PhLevelinputData"},
	{Name: "TurbidityLevel", DTY: "tbs", Value: "Turbidity", Code: "tb", List: "TurbidityLevelData", XMLRoot: "TurbidityLevelinput", XMLItem: "TurbidityLevelData"},
	{Name: "LevelSensor", DTY: "sl", Value: "LevelSensorValue", Code: "lm", List: "LevelSensorData", XMLRoot: "LevelSensorinput", XMLItem: "LevelSensorData"},
	{Name: "Pressure", DTY: "ps", Value: "Pressure", Code: "pr", List: "PressureSensorData", XMLRoot: "Pressureinput", XMLItem: "PressureSensorData", Retired: true},
	{Name: "Nitrate", DTY: "no3", Value: "NitrateValue", Code: "no3", List: "NitrateData", Unlisted: true},
	{Name: "Fluoride", DTY: "fl", Value: "FluorideValue", Code: "fl", List: "FluorideData", Unlisted: true},
	{Name: "Iron", DTY: "fe", Value: "IronValue", Code: "fe", List: "IronData", Unlisted: true},
	{Name: "Salinity", DTY: "sal", Value: "SalinityValue", Code: "sal", List: "SalinityData", Unlisted: true},
	{Name: "Arsenic", DTY: "as", Value: "ArsenicValue", Code: "as", List: "ArsenicData", Unlisted: true},
	{Name: "TDS", DTY: "tds", Value: "TDSValue", Code: "tds", List: "TDSData", Unlisted: true},
}

// EjalshaktiAPI finds an API by name (any case).
func EjalshaktiAPI(name string) (PortalAPI, bool) {
	for _, a := range PortalAPIs {
		if strings.EqualFold(a.Name, name) {
			return a, true
		}
	}
	return PortalAPI{}, false
}

// EjalshaktiAPIOf is the API a device type's readings go to.
func EjalshaktiAPIOf(dty string) (PortalAPI, bool) {
	for _, a := range PortalAPIs {
		if a.DTY == dty {
			return a, true
		}
	}
	return PortalAPI{}, false
}

func EjalshaktiNames() []string {
	out := make([]string, len(PortalAPIs))
	for i, a := range PortalAPIs {
		out[i] = a.Name
	}
	return out
}

// EJError is a refusal with the HTTP status to answer.
type EJError struct {
	Code int
	Msg  string
}

func (e EJError) Error() string { return e.Msg }

var didClean = regexp.MustCompile(`[^A-Za-z0-9._~-]+`)

// EJPayloads turns one API body into report payloads: gid is the VillageId,
// did the GatewayId (characters other than letters, digits and . _ ~ -
// become _) and the device type, so a gateway's sensors are separate
// devices.
func EJPayloads(api PortalAPI, b []byte, contentType string) ([]Payload, error) {
	body, err := ejBody(b, contentType)
	if err != nil {
		return nil, EJError{http.StatusBadRequest, err.Error()}
	}
	village, gateway := field(body, "VillageId"), field(body, "GatewayId")
	if village == "" || gateway == "" {
		return nil, EJError{http.StatusBadRequest, "VillageId and GatewayId are required."}
	}
	gid := didClean.ReplaceAllString(village, "_")
	did := strings.Trim(didClean.ReplaceAllString(gateway, "_"), "_") + "-" + api.DTY
	if len(did) > 64 {
		did = did[len(did)-64:]
	}
	mk := func(dt string, p map[string]any) Payload {
		return Payload{GID: gid, DID: did, DTY: api.DTY, DT: dt, P: p, VillageID: village, GatewayID: gateway}
	}
	var out []Payload
	if api.Daily {
		p := map[string]any{}
		if v, ok := member(body, api.Value); ok {
			p[api.Code] = v
		}
		if api.Value2 != "" {
			if v, ok := member(body, api.Value2); ok {
				p[api.Code2] = v
			}
		}
		if len(p) == 0 {
			return nil, EJError{http.StatusBadRequest, api.Value + " is required."}
		}
		return []Payload{mk(field(body, "Date"), p)}, nil
	}
	if v, ok := member(body, api.Value); ok {
		out = append(out, mk(field(body, "TimeStamp"), map[string]any{api.Code: v}))
	}
	if l, ok := member(body, api.List); ok {
		items, _ := l.([]any)
		if m, isObj := l.(map[string]any); isObj {
			// XML: the collection element wraps entries of their own name
			// (<ChlorineData><ChlorineData>…</ChlorineData></ChlorineData>,
			// <PhLevelData><PhLevelinputData>…</PhLevelinputData></PhLevelData>);
			// a lone object is one entry.
			if _, direct := member(m, api.Value); direct {
				items = []any{m}
			} else {
				for _, k := range sortedKeys(m) {
					items = append(items, asList(m[k])...)
				}
			}
		}
		for _, it := range items {
			m, _ := it.(map[string]any)
			if v, ok := member(m, api.Value); ok {
				out = append(out, mk(field(m, "TimeStamp"), map[string]any{api.Code: v}))
			}
		}
	}
	if len(out) == 0 {
		return nil, EJError{http.StatusBadRequest, api.Value + " or " + api.List + " is required."}
	}
	return out, nil
}

func asList(v any) []any {
	switch x := v.(type) {
	case []any:
		return x
	case map[string]any:
		return []any{x}
	}
	return nil
}

// member finds a member without regard to case or surrounding spaces (the
// manual's pH sample has a key with a leading space, F-2705).
func member(m map[string]any, key string) (any, bool) {
	if v, ok := m[key]; ok {
		return v, true
	}
	for k, v := range m {
		if strings.EqualFold(strings.TrimSpace(k), key) {
			return v, true
		}
	}
	return nil, false
}

func field(m map[string]any, key string) string {
	v, _ := member(m, key)
	s, _ := asText(v)
	return s
}

// ejBody reads a JSON or DataContract XML body into a map.
func ejBody(b []byte, contentType string) (map[string]any, error) {
	t := bytes.TrimSpace(b)
	if strings.Contains(contentType, "xml") || (len(t) > 0 && t[0] == '<') {
		return xmlBody(t)
	}
	dec := json.NewDecoder(bytes.NewReader(t))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("the body is not a JSON object: %v", err)
	}
	return m, nil
}

// xmlBody reads <XInput><A>1</A><L><L><B>2</B></L></L></XInput> as
// {"A": "1", "L": {"L": {"B": "2"}}} (repeated elements become a list).
func xmlBody(b []byte) (map[string]any, error) {
	dec := xml.NewDecoder(bytes.NewReader(b))
	type frame struct {
		name string
		m    map[string]any
		text strings.Builder
	}
	var stack []*frame
	var root map[string]any
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("the body is not XML: %v", err)
		}
		switch x := tok.(type) {
		case xml.StartElement:
			stack = append(stack, &frame{name: x.Name.Local, m: map[string]any{}})
		case xml.CharData:
			if len(stack) > 0 {
				stack[len(stack)-1].text.Write(x)
			}
		case xml.EndElement:
			f := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			var v any = f.m
			if len(f.m) == 0 {
				v = strings.TrimSpace(f.text.String())
			}
			if len(stack) == 0 {
				root = f.m
				continue
			}
			parent := stack[len(stack)-1].m
			if prev, ok := parent[f.name]; ok {
				if l, isList := prev.([]any); isList {
					parent[f.name] = append(l, v)
				} else {
					parent[f.name] = []any{prev, v}
				}
			} else {
				parent[f.name] = v
			}
		}
	}
	if root == nil {
		return nil, errors.New("the body is empty")
	}
	return root, nil
}

// CentralEnvelope is a queued push in the ejalshakti format.
type CentralEnvelope struct {
	API  string         `json:"api"`
	Body map[string]any `json:"body"`
}

// ErrKeyRefused is the portal refusing an API's key: that API's pushes
// wait while the others go on.
type ErrKeyRefused struct{ API, Msg string }

func (e ErrKeyRefused) Error() string { return "ejalshakti refused the key of " + e.API + ": " + e.Msg }

// QueuedAPI is the API of a queued ejalshakti push ("" for the report
// format).
func QueuedAPI(payload []byte) string {
	var env CentralEnvelope
	if json.Unmarshal(payload, &env) != nil {
		return ""
	}
	return env.API
}
