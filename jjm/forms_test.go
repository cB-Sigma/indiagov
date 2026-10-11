// Copyright 2026 Abhinav Gautam
// SPDX-License-Identifier: Apache-2.0

package jjm_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cB-Sigma/indiagov/jjm"
)

// Bodies written as DataContract XML (EJXML) read back with EJPayloads
// for every API with a published XML form; the XML has the help pages'
// root and item elements, members in alphabetical order.
func TestEJXMLReadsBack(t *testing.T) {
	at := time.Date(2026, 10, 11, 6, 0, 0, 0, ist)
	for _, a := range jjm.PortalAPIs {
		if a.XMLRoot == "" {
			if _, err := jjm.EJXML(a, map[string]any{}); err == nil {
				t.Errorf("%s: XML without a published form", a.Name)
			}
			continue
		}
		var body map[string]any
		if a.Daily {
			body, _ = jjm.EJDailyBody(a, "2730100123", "GW-DEMO-01", "2026-10-10", 42.5, 55)
		} else {
			body, _ = jjm.EJSeriesBody(a, "2730100123", "GW-DEMO-01", ist, jjm.EJReading{Time: at, Value: 0.4}, jjm.EJReading{Time: at.Add(time.Minute), Value: 0.5})
		}
		x, err := jjm.EJXML(a, body)
		if err != nil {
			t.Fatal(err)
		}
		s := string(x)
		if !strings.HasPrefix(s, "<"+a.XMLRoot+" ") || !strings.Contains(s, "IoT_API.Models") {
			t.Errorf("%s: %s", a.Name, s)
		}
		if !a.Daily && strings.Count(s, "<"+a.XMLItem+">") < 2 {
			t.Errorf("%s: entries are not %s: %s", a.Name, a.XMLItem, s)
		}
		if strings.Index(s, "<GatewayId>") > strings.Index(s, "<VillageId>") {
			t.Errorf("%s: members out of order: %s", a.Name, s)
		}
		ps, err := jjm.EJPayloads(a, x, "application/xml")
		want := 2
		if a.Daily {
			want = 1
		}
		if err != nil || len(ps) != want {
			t.Errorf("%s: read back %d readings, %v\n%s", a.Name, len(ps), err, s)
		}
	}
}

// The pH collection's XML entries are named PhLevelinputData (the help
// page's sample), not PhLevelData.
func TestEJXMLPHItems(t *testing.T) {
	ph, _ := jjm.EjalshaktiAPI("PHLevel")
	b := []byte(`<PhLevelinput xmlns="http://schemas.datacontract.org/2004/07/IoT_API.Models"><GatewayId>G1</GatewayId>
<PhLevelData><PhLevelinputData><PH_Level>7.1</PH_Level><TimeStamp>2026-10-11T06:00:00.0000000+05:30</TimeStamp></PhLevelinputData>
<PhLevelinputData><PH_Level>7.3</PH_Level><TimeStamp>2026-10-11T06:15:00.0000000+05:30</TimeStamp></PhLevelinputData></PhLevelData>
<VillageId>2730100123</VillageId></PhLevelinput>`)
	ps, err := jjm.EJPayloads(ph, b, "text/xml")
	if err != nil || len(ps) != 2 || ps[1].P["ph"] != "7.3" {
		t.Errorf("%+v, %v", ps, err)
	}
}

// Both shapes of a series body are read: the collection only (the
// manuals) and a reading at the top level beside it (the help pages);
// member names in any case and with stray spaces; both date forms.
func TestEJShapesAndForms(t *testing.T) {
	cl, _ := jjm.EjalshaktiAPI("Chlorine")
	at := time.Date(2026, 10, 11, 6, 0, 0, 0, ist)
	body, _ := jjm.EJSeriesBody(cl, "2730100123", "GW-DEMO-01", ist, jjm.EJReading{Time: at, Value: 0.4}, jjm.EJReading{Time: at.Add(time.Minute), Value: 0.5})
	top := jjm.EJTopLevel(cl, body)
	if top["ResidualCL"] != 0.4 || len(top["ChlorineData"].([]any)) != 1 {
		t.Errorf("top level %v", top)
	}
	for name, b := range map[string]map[string]any{"collection": body, "top level": top} {
		raw, _ := json.Marshal(b)
		if ps, err := jjm.EJPayloads(cl, raw, ""); err != nil || len(ps) != 2 {
			t.Errorf("%s: %d, %v", name, len(ps), err)
		}
	}
	ph, _ := jjm.EjalshaktiAPI("phlevel")
	ps, err := jjm.EJPayloads(ph, []byte(`{"villageid":"2730100123","GATEWAYID":"G","PhLevelData":[{"TimeStamp":"2026-10-11T06:00:00","  PH_Level":7.2}]}`), "")
	if err != nil || len(ps) != 1 || ps[0].VillageID != "2730100123" {
		t.Errorf("keys in any case: %+v, %v", ps, err)
	}
	fm, _ := jjm.EjalshaktiAPI("FlowMeter")
	for _, date := range []string{"2026-10-10", "2026-10-10T00:00:00.000+05:30"} {
		ps, err := jjm.EJPayloads(fm, []byte(`{"VillageId":"2730100123","GatewayId":"G","WaterSupply":42.5,"LPCD":55,"Date":"`+date+`"}`), "")
		if err != nil {
			t.Fatal(err)
		}
		r, err := jjm.Decoder{Loc: ist}.Check(ps[0])
		if err != nil || !r.HasTime || r.Time.Format("2006-01-02") != "2026-10-10" {
			t.Errorf("Date %s: %+v, %v", date, r, err)
		}
	}
	// A device message with its members in upper case.
	r, err := jjm.Decoder{Loc: ist}.Decode([]byte(`{"GID":"G_1_2_3","DID":"fm1","DTY":"FM","DT":"2026-10-11T05:00:00","P":{"T":"5"}}`))
	if err != nil || r.DTY != "fm" || r.Values["quantity"] != 5.0 {
		t.Errorf("upper case: %+v, %v", r, err)
	}
}

// Retired and unlisted APIs are read, but EJClient sends to them only
// with AllowRetired.
func TestEJRetired(t *testing.T) {
	gw, _ := jjm.EjalshaktiAPI("GroundWater")
	pr, _ := jjm.EjalshaktiAPI("Pressure")
	tds, _ := jjm.EjalshaktiAPI("TDS")
	if gw.Current() || pr.Current() || tds.Current() || !gw.Retired || !tds.Unlisted {
		t.Fatal("flags")
	}
	current := 0
	for _, a := range jjm.PortalAPIs {
		if a.Current() {
			current++
		}
	}
	if current != 5 {
		t.Errorf("%d current APIs, want the help pages' five", current)
	}
	if ps, err := jjm.EJPayloads(gw, []byte(`{"VillageId":"v","GatewayId":"g","Depth":12,"Date":"2026-10-10"}`), ""); err != nil || len(ps) != 1 {
		t.Errorf("GroundWater read: %v", err)
	}
	if ps, err := jjm.EJPayloads(pr, []byte(`{"VillageId":"v","GatewayId":"g","PressureSensorData":[{"TimeStamp":"2026-10-10T06:00:00","Pressure":2.5}]}`), ""); err != nil || len(ps) != 1 {
		t.Errorf("Pressure read: %v", err)
	}
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = w.Write([]byte(`{"Status":true,"Message":"sucess"}`))
	}))
	defer srv.Close()
	c := jjm.EJClient{BaseURL: srv.URL, Key: "k"}
	body, _ := jjm.EJDailyBody(gw, "v", "g", "2026-10-10", 12, 0)
	var pe *jjm.PushError
	if _, err := c.Push(context.Background(), "GroundWater", body); !errors.As(err, &pe) || pe.Retry || hits != 0 {
		t.Errorf("retired push: %v, %d requests", err, hits)
	}
	c.AllowRetired = true
	if _, err := c.Push(context.Background(), "GroundWater", body); err != nil || hits != 1 {
		t.Errorf("allowed: %v", err)
	}
	c.XML = true
	cl, _ := jjm.EjalshaktiAPI("Chlorine")
	b2, _ := jjm.EJSeriesBody(cl, "v", "g", ist, jjm.EJReading{Time: time.Now(), Value: 0.3})
	if _, err := c.Push(context.Background(), "Chlorine", b2); err != nil || hits != 2 {
		t.Errorf("XML push: %v", err)
	}
}
