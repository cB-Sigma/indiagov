// Copyright 2026 Abhinav Gautam
// SPDX-License-Identifier: Apache-2.0

package jjm_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/cB-Sigma/indiagov/jjm"
)

var ist = time.FixedZone("IST", 19800)

// A message built with NewPayload passes the decoder with the values it
// was given; ground water in feet is converted to metres.
func TestNewPayloadDecodes(t *testing.T) {
	at := time.Date(2026, 10, 11, 6, 0, 0, 0, ist)
	for dty, dt := range jjm.DeviceTypes {
		vals := map[string]any{}
		for _, code := range dt.Params {
			if code == "e" {
				vals[code] = jjm.ErrorNone
				continue
			}
			vals[code] = 1.5
			if code == "s" {
				vals[code] = 1.0 // pump status is 0 or 1
			}
		}
		p, err := jjm.NewPayload("MH_D01_B02_V03", dty+"01", dty, at, ist, vals)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := p.JSON()
		r, err := jjm.Decoder{Loc: ist, Now: func() time.Time { return at }}.Decode(b)
		if err != nil {
			t.Fatalf("%s: %v", dty, err)
		}
		if !r.Time.Equal(at) || r.Quality() != "good" {
			t.Errorf("%s: %s, time %v, quality %s", dty, b, r.Time, r.Quality())
		}
		for _, code := range dt.Params {
			if code == "e" {
				continue
			}
			want := vals[code].(float64)
			if code == "g" {
				want = 0.4572 // 1.5 ft
			}
			if got := r.Values[jjm.Params[code].Name]; got != want {
				t.Errorf("%s: %s = %v, want %v", dty, code, got, want)
			}
		}
	}
	if _, err := jjm.NewPayload("bad gid", "d", "fm", at, ist, nil); err == nil {
		t.Error("a gid with a space was taken")
	}
	if _, err := jjm.NewPayload("G", "d", "fm", at, ist, map[string]any{"t": []int{1}}); err == nil {
		t.Error("a list value was taken")
	}
}

// Faults in device messages: error codes, ranges, unknown codes, a value
// that is no number, no time, a future time.
func TestDecoderFlags(t *testing.T) {
	now := time.Date(2026, 10, 11, 6, 0, 0, 0, ist)
	dec := jjm.Decoder{Loc: ist, Now: func() time.Time { return now }}
	cases := []struct{ msg, flags string }{
		{`{"gid":"G_1_2_3","did":"fm1","dty":"fm","dt":"2026-10-11T05:00:00","p":{"t":"5","e":"01"}}`, "error:tamper"},
		{`{"gid":"G_1_2_3","did":"ca1","dty":"ca","dt":"2026-10-11T05:00:00","p":{"c":"12"}}`, "range:c"},
		{`{"gid":"G_1_2_3","did":"ph1","dty":"ph","dt":"2026-10-11T05:00:00","p":{"ph":"x"}}`, "invalid:ph"},
		{`{"gid":"G_1_2_3","did":"ph1","dty":"ph","dt":"2026-10-11T05:00:00","p":{"ph":[1]}}`, "invalid:ph"},
		{`{"gid":"G_1_2_3","did":"ps1","dty":"ps","dt":"","p":{"pr":"2"}}`, "no-time"},
		{`{"gid":"G_1_2_3","did":"ps1","dty":"ps","dt":"soon","p":{"pr":"2"}}`, "no-time"},
		{`{"gid":"G_1_2_3","did":"ps1","dty":"ps","dt":"2026-10-12T05:00:00","p":{"pr":"2"}}`, "future"},
		{`{"gid":"G_1_2_3","did":"ca1","dty":"ca","dt":"2026-10-11T05:00:00","p":{"c":"1","e":"11"}}`, "unknown-code:11"},
		{`{"gid":"G_1_2_3","did":"fm1","dty":"fm","dt":"2026-10-11T05:00:00","p":{"t":"1","e":"11"}}`, "good"},
	}
	for _, c := range cases {
		r, err := dec.Decode([]byte(c.msg))
		if err != nil {
			t.Fatalf("%s: %v", c.msg, err)
		}
		if r.Quality() != c.flags {
			t.Errorf("%s: quality %s, want %s", c.msg, r.Quality(), c.flags)
		}
	}
	for _, bad := range []string{`{"gid":"","did":"x"}`, `{"gid":"G","did":"a b"}`, `[1]`, `{"gid":`} {
		if _, err := dec.Decode([]byte(bad)); err == nil {
			t.Errorf("%s was taken", bad)
		}
	}
	r, _ := jjm.Decoder{Loc: ist, FlowErrorCodes: "section5"}.Decode([]byte(`{"gid":"G","did":"fm1","dty":"fm","dt":"2026-10-11T05:00:00","p":{"e":"01"}}`))
	if r.Error != "leakage" {
		t.Errorf("section 5(f) code 01 is %q", r.Error)
	}
}

func TestMethods(t *testing.T) {
	if jjm.Method("fm") != "FlowmeterDataInsert" || jjm.Method("zz") != "DataInsert" {
		t.Errorf("methods %s %s", jjm.Method("fm"), jjm.Method("zz"))
	}
	for dty := range jjm.DeviceTypes {
		if got, ok := jjm.MethodDTY(jjm.Method(dty)); !ok || got != dty {
			t.Errorf("MethodDTY(Method(%s)) = %s, %v", dty, got, ok)
		}
	}
	if d, ok := jjm.MethodDTY("DataInsert"); !ok || d != "" {
		t.Error("DataInsert")
	}
	if _, ok := jjm.MethodDTY("Flowmeter"); ok {
		t.Error("a name without DataInsert was taken")
	}
	if jjm.Interval("fm", "battery") != 12*time.Hour || jjm.Interval("fm", "mains") != 15*time.Minute || jjm.Interval("zz", "") != 0 {
		t.Error("intervals")
	}
	lat := 95.0
	if err := (jjm.Metadata{GID: "G", DID: "d", DTY: "fm", Latitude: &lat}).Check(); err == nil {
		t.Error("latitude 95 was taken")
	}
	if err := (jjm.Metadata{GID: "G", DID: "d", DTY: "fm", Power: "solar"}).Check(); err != nil {
		t.Error(err)
	}
}

// Central.Post against a server answering each way the API can.
func TestCentralPost(t *testing.T) {
	var gotPath, gotKey string
	answer := func(w http.ResponseWriter) {}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotKey = r.URL.Path, r.Header.Get("X-API-Key")
		if r.Header.Get("Authorization") != "" {
			gotKey = r.Header.Get("Authorization")
		}
		answer(w)
	}))
	defer srv.Close()
	p, _ := jjm.NewPayload("MH_D02_B03_V04", "fm01", "fm", time.Now(), ist, map[string]any{"t": 1})
	c := jjm.Central{BaseURL: srv.URL + "/", Key: "demo-key"}
	ctx := context.Background()

	answer = func(w http.ResponseWriter) {
		_, _ = io.WriteString(w, `{"Status":"1","Message":" Data uploaded successfully.","DeviceID":"fm01","UploadedOn":" 2026-10-11T06:00:00"}`)
	}
	r, err := c.Post(ctx, p)
	if err != nil || !r.OK() || r.Message != jjm.CentralSuccess || gotPath != "/FlowmeterDataInsert" || gotKey != "demo-key" {
		t.Errorf("success: %+v, %v, %s, %s", r, err, gotPath, gotKey)
	}
	c.Auth = "bearer"
	_, _ = c.Post(ctx, p)
	if gotKey != "Bearer demo-key" {
		t.Errorf("bearer: %s", gotKey)
	}
	for _, tc := range []struct {
		name  string
		code  int
		body  string
		retry bool
	}{
		{"status 0", 200, `{"Status":"0","Message":"Try later."}`, true},
		{"not JSON", 200, `<html>`, true},
		{"server down", 503, ``, true},
		{"key refused", 401, `{"Status":"0","Message":"Invalid key"}`, true},
		{"bad request", 400, `{"Status":"0","Message":"Invalid data"}`, false},
	} {
		answer = func(w http.ResponseWriter) { w.WriteHeader(tc.code); _, _ = io.WriteString(w, tc.body) }
		_, err := c.Post(ctx, p)
		var pe *jjm.PushError
		if !errors.As(err, &pe) || pe.Retry != tc.retry || jjm.ErrRetry(err) != tc.retry {
			t.Errorf("%s: %v", tc.name, err)
		}
	}
	srv.Close()
	if _, err := c.Post(ctx, p); err == nil || !jjm.ErrRetry(err) {
		t.Errorf("unreachable: %v", err)
	}
}

// Bodies written with EJSeriesBody and EJDailyBody read back with
// EJPayloads, for every API.
func TestEJBodiesReadBack(t *testing.T) {
	at := time.Date(2026, 10, 11, 6, 0, 0, 0, ist)
	for _, api := range jjm.PortalAPIs {
		var body map[string]any
		var err error
		if api.Daily {
			body, err = jjm.EJDailyBody(api, "2730100123", "GW-DEMO-01", "2026-10-10", 42, 55)
		} else {
			body, err = jjm.EJSeriesBody(api, "2730100123", "GW-DEMO-01", ist, jjm.EJReading{Time: at, Value: 0.4}, jjm.EJReading{Time: at.Add(time.Minute), Value: 0.5})
		}
		if err != nil {
			t.Fatalf("%s: %v", api.Name, err)
		}
		b, _ := json.Marshal(body)
		ps, err := jjm.EJPayloads(api, b, "application/json")
		if err != nil {
			t.Fatalf("%s: %s: %v", api.Name, b, err)
		}
		want := 2
		if api.Daily {
			want = 1
		}
		if len(ps) != want || ps[0].DTY != api.DTY || ps[0].VillageID != "2730100123" || ps[0].DID != "GW-DEMO-01-"+api.DTY {
			t.Errorf("%s: %+v", api.Name, ps)
		}
		r, err := jjm.Decoder{Loc: ist}.Check(ps[0])
		if err != nil || !r.HasTime {
			t.Errorf("%s: reading %+v, %v", api.Name, r, err)
		}
	}
	fm, _ := jjm.EjalshaktiAPI("flowmeter")
	if _, err := jjm.EJSeriesBody(fm, "v", "g", ist, jjm.EJReading{}); err == nil {
		t.Error("a series body for a daily API")
	}
	cl, _ := jjm.EjalshaktiAPI("Chlorine")
	if _, err := jjm.EJDailyBody(cl, "v", "g", "2026-10-10", 1, 0); err == nil {
		t.Error("a daily body for a series API")
	}
	if _, err := jjm.EJDailyBody(fm, "v", "g", "10/10/2026", 1, 0); err == nil {
		t.Error("a bad date")
	}
	if _, err := jjm.EJSeriesBody(cl, "", "g", ist, jjm.EJReading{}); err == nil {
		t.Error("no VillageId")
	}
}

// The body readers refuse what the portal would refuse.
func TestEJPayloadsRefuse(t *testing.T) {
	cl, _ := jjm.EjalshaktiAPI("Chlorine")
	for _, b := range []string{`{"GatewayId":"g","ResidualCL":1}`, `{"VillageId":"v","GatewayId":"g"}`, `{"VillageId":`, `<a><b></a>`} {
		_, err := jjm.EJPayloads(cl, []byte(b), "")
		var e jjm.EJError
		if !errors.As(err, &e) || e.Code != 400 {
			t.Errorf("%s: %v", b, err)
		}
	}
	if jjm.QueuedAPI([]byte(`{"api":"Chlorine","body":{}}`)) != "Chlorine" || jjm.QueuedAPI([]byte(`x`)) != "" {
		t.Error("QueuedAPI")
	}
}

func TestEJClient(t *testing.T) {
	var auth, path string
	answer := `{"Status":true,"Message":"sucess"}`
	code := 200
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth, path = r.Header.Get("Authorization"), r.URL.Path
		w.WriteHeader(code)
		_, _ = io.WriteString(w, answer)
	}))
	defer srv.Close()
	c := jjm.EJClient{BaseURL: srv.URL + "/IoT_API/API", Keys: map[string]string{"chlorine": "k-cl"}, Key: "k-any"}
	body := map[string]any{"VillageId": "v", "GatewayId": "g", "ResidualCL": 0.3}
	if _, err := c.Push(context.Background(), "Chlorine", body); err != nil || auth != "k-cl" || path != "/IoT_API/API/Chlorine" {
		t.Errorf("push: %v, %s, %s", err, auth, path)
	}
	env, _ := json.Marshal(jjm.CentralEnvelope{API: "LevelSensor", Body: body})
	if _, err := c.PushQueued(context.Background(), env); err != nil || auth != "k-any" {
		t.Errorf("queued push: %v, %s", err, auth)
	}
	answer = `{"Status":true,"Message":"KeyToken not Matched"}`
	_, err := c.Push(context.Background(), "Chlorine", body)
	var kr jjm.ErrKeyRefused
	if !errors.As(err, &kr) || kr.API != "Chlorine" || !jjm.ErrRetry(err) {
		t.Errorf("key refused: %v", err)
	}
	answer, code = `{"Message":"The request is invalid."}`, 400
	_, err = c.Push(context.Background(), "Chlorine", body)
	var pe *jjm.PushError
	if !errors.As(err, &pe) || pe.Retry {
		t.Errorf("bad request: %v", err)
	}
	answer, code = ``, 500
	if _, err := c.Push(context.Background(), "Chlorine", body); !jjm.ErrRetry(err) {
		t.Errorf("server error: %v", err)
	}
	answer, code = `{"Status":false,"Message":"x"}`, 200
	if _, err := c.Push(context.Background(), "Chlorine", body); !jjm.ErrRetry(err) {
		t.Errorf("Status false: %v", err)
	}
	if _, err := c.Push(context.Background(), "Nope", body); err == nil {
		t.Error("an unknown API was pushed")
	}
	if !slices.Contains(jjm.EjalshaktiNames(), "TDS") {
		t.Error("names")
	}
}
