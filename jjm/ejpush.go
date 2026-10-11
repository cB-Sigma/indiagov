// Copyright 2026 Abhinav Gautam
// SPDX-License-Identifier: Apache-2.0

package jjm

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Writing to the eJalShakti IoT push API (see ejalshakti.go for the API
// table and the readers).

// EJTimeLayout is the TimeStamp layout of the help pages' samples
// (seven fractional digits and the offset).
const EJTimeLayout = "2006-01-02T15:04:05.0000000-07:00"

// EJKeyNotMatched is the message the portal answers a wrong key with
// (with Status true).
const EJKeyNotMatched = "KeyToken not Matched"

// EJReading is one timed value of a series API.
type EJReading struct {
	Time  time.Time
	Value float64
}

// EJSeriesBody is a series API's body (Chlorine, PHLevel, ...) with every
// reading in the API's collection member, the shape of the manuals'
// samples. loc is the time zone TimeStamp is written in.
func EJSeriesBody(api PortalAPI, villageID, gatewayID string, loc *time.Location, readings ...EJReading) (map[string]any, error) {
	if api.Daily {
		return nil, fmt.Errorf("%s takes one value per day; use EJDailyBody", api.Name)
	}
	if villageID == "" || gatewayID == "" {
		return nil, errors.New("VillageId and GatewayId are required")
	}
	if len(readings) == 0 {
		return nil, errors.New("no reading")
	}
	if loc == nil {
		loc = time.UTC
	}
	l := make([]any, 0, len(readings))
	for _, r := range readings {
		l = append(l, map[string]any{"TimeStamp": r.Time.In(loc).Format(EJTimeLayout), api.Value: r.Value})
	}
	return map[string]any{"VillageId": villageID, "GatewayId": gatewayID, api.List: l}, nil
}

// EJTopLevel moves a series body's first collection entry to the top
// level, the other shape the help pages show (a reading beside the
// collection); a body with one reading then has no collection.
func EJTopLevel(api PortalAPI, body map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range body {
		out[k] = v
	}
	l, _ := out[api.List].([]any)
	if len(l) == 0 {
		return out
	}
	first, _ := l[0].(map[string]any)
	for k, v := range first {
		out[k] = v
	}
	if len(l) == 1 {
		delete(out, api.List)
	} else {
		out[api.List] = l[1:]
	}
	return out
}

// EJXML writes a body as the DataContract XML the help pages show: the
// API's root element in the IoT_API.Models namespace, members in
// alphabetical order (DataContract's default), each collection entry in
// the API's item element.
func EJXML(api PortalAPI, body map[string]any) ([]byte, error) {
	if api.XMLRoot == "" {
		return nil, fmt.Errorf("no XML form is published for %s", api.Name)
	}
	var b bytes.Buffer
	b.WriteString("<" + api.XMLRoot + ` xmlns:i="http://www.w3.org/2001/XMLSchema-instance" xmlns="http://schemas.datacontract.org/2004/07/IoT_API.Models">`)
	var write func(m map[string]any) error
	write = func(m map[string]any) error {
		for _, k := range sortedKeys(m) {
			b.WriteString("<" + k + ">")
			switch x := m[k].(type) {
			case []any:
				for _, it := range x {
					e, ok := it.(map[string]any)
					if !ok {
						return fmt.Errorf("%s: an entry is not an object", k)
					}
					b.WriteString("<" + api.XMLItem + ">")
					if err := write(e); err != nil {
						return err
					}
					b.WriteString("</" + api.XMLItem + ">")
				}
			case map[string]any:
				return fmt.Errorf("%s: nested objects are not written", k)
			default:
				s, ok := asText(x)
				if !ok {
					s = fmt.Sprint(x)
				}
				if err := xml.EscapeText(&b, []byte(s)); err != nil {
					return err
				}
			}
			b.WriteString("</" + k + ">")
		}
		return nil
	}
	if err := write(body); err != nil {
		return nil, err
	}
	b.WriteString("</" + api.XMLRoot + ">")
	return b.Bytes(), nil
}

// EJDailyBody is a daily API's body (FlowMeter: WaterSupply and LPCD;
// GroundWater: Depth) for date (YYYY-MM-DD). value2 is FlowMeter's LPCD
// and ignored otherwise.
func EJDailyBody(api PortalAPI, villageID, gatewayID, date string, value, value2 float64) (map[string]any, error) {
	if !api.Daily {
		return nil, fmt.Errorf("%s takes timed readings; use EJSeriesBody", api.Name)
	}
	if villageID == "" || gatewayID == "" {
		return nil, errors.New("VillageId and GatewayId are required")
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return nil, errors.New("the date must be YYYY-MM-DD")
	}
	b := map[string]any{"VillageId": villageID, "GatewayId": gatewayID, "Date": date, api.Value: value}
	if api.Value2 != "" {
		b[api.Value2] = value2
	}
	return b, nil
}

// EJResponse is the portal's answer {"Status", "Message"}; Status is a
// boolean on the portal and may come as text from look-alike servers.
type EJResponse struct {
	Status  any    `json:"Status"`
	Message string `json:"Message"`
}

// OK reports Status true (or "true", or 1) without the key-refused
// message.
func (r EJResponse) OK() bool {
	s := strings.ToLower(fmt.Sprint(r.Status))
	return (s == "true" || s == "1") && !keyNotMatched(r.Message)
}

func keyNotMatched(msg string) bool {
	return strings.Contains(strings.ToLower(msg), "not matched")
}

// ReadEJResponse classifies an answer of the portal: nil for success,
// ErrKeyRefused for a key the portal does not take (it answers Status
// true with EJKeyNotMatched; 401 and 403 count too), a *PushError
// otherwise.
func ReadEJResponse(api string, httpStatus int, body []byte) (EJResponse, error) {
	var r EJResponse
	jsonErr := json.Unmarshal(body, &r)
	r.Message = strings.TrimSpace(r.Message)
	switch {
	case httpStatus >= 500 || httpStatus == http.StatusTooManyRequests:
		return r, &PushError{HTTPStatus: httpStatus, Message: msgOr(r.Message, http.StatusText(httpStatus)), Retry: true}
	case httpStatus == http.StatusUnauthorized || httpStatus == http.StatusForbidden || keyNotMatched(r.Message):
		return r, ErrKeyRefused{API: api, Msg: msgOr(r.Message, http.StatusText(httpStatus))}
	case httpStatus >= 400:
		return r, &PushError{HTTPStatus: httpStatus, Message: msgOr(r.Message, http.StatusText(httpStatus))}
	case jsonErr != nil:
		return r, &PushError{HTTPStatus: httpStatus, Message: "the answer is not JSON: " + jsonErr.Error(), Retry: true}
	case !r.OK():
		return r, &PushError{HTTPStatus: httpStatus, Message: fmt.Sprintf("Status %v: %s", r.Status, r.Message), Retry: true}
	}
	return r, nil
}

// EJClient pushes to the portal (or a server of the same API).
type EJClient struct {
	// BaseURL defaults to EjalshaktiURL.
	BaseURL string
	// Keys are the portal's keys by API name; Key is used for an API
	// without one.
	Keys   map[string]string
	Key    string
	Client *http.Client
	// XML sends DataContract XML bodies (EJXML) instead of JSON.
	XML bool
	// AllowRetired lets Push send to APIs the help pages no longer list
	// (PortalAPI.Retired or Unlisted), for a server that still takes them.
	AllowRetired bool
}

// Push posts body to api with that API's key in Authorization, as it is.
func (c EJClient) Push(ctx context.Context, api string, body map[string]any) (EJResponse, error) {
	a, ok := EjalshaktiAPI(api)
	if !ok {
		return EJResponse{}, fmt.Errorf("%q is not an eJalShakti API (%s)", api, strings.Join(EjalshaktiNames(), ", "))
	}
	if !a.Current() && !c.AllowRetired {
		return EJResponse{}, &PushError{Message: a.Name + " is not listed by the portal's help pages any more (set AllowRetired to send it)"}
	}
	ct := "application/json"
	b, err := json.Marshal(body)
	if c.XML {
		ct = "application/xml"
		b, err = EJXML(a, body)
	}
	if err != nil {
		return EJResponse{}, err
	}
	base := c.BaseURL
	if base == "" {
		base = EjalshaktiURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(base, "/")+"/"+a.Name, bytes.NewReader(b))
	if err != nil {
		return EJResponse{}, err
	}
	req.Header.Set("Content-Type", ct)
	if k := c.key(a.Name); k != "" {
		req.Header.Set("Authorization", k)
	}
	client := c.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return EJResponse{}, &PushError{Message: err.Error(), Retry: true}
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	return ReadEJResponse(a.Name, resp.StatusCode, rb)
}

// PushQueued posts a CentralEnvelope (a queued push).
func (c EJClient) PushQueued(ctx context.Context, envelope []byte) (EJResponse, error) {
	var env CentralEnvelope
	if err := json.Unmarshal(envelope, &env); err != nil || env.API == "" {
		return EJResponse{}, &PushError{Message: "not a queued eJalShakti push"}
	}
	return c.Push(ctx, env.API, env.Body)
}

func (c EJClient) key(api string) string {
	for k, v := range c.Keys {
		if strings.EqualFold(k, api) {
			return v
		}
	}
	return c.Key
}
