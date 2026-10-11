// Copyright 2026 Abhinav Gautam
// SPDX-License-Identifier: Apache-2.0

package pmkusum_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/cB-Sigma/indiagov/pmkusum"
)

// demoCert makes a throw-away certificate at run time.
func demoCert(t *testing.T, notAfter time.Time) string {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "860000000000001"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: notAfter}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &k.PublicKey, k)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

func TestRegistration(t *testing.T) {
	exp := time.Now().Add(365 * 24 * time.Hour).Truncate(time.Second)
	r := pmkusum.Registration{IMEI: "860000000000001", Certificate: demoCert(t, exp), BrokerURL: "mqtts://rms.example.invalid:8883",
		InfoTopic: pmkusum.Topic("", "Standalonesolarpump", "860000000000001", pmkusum.TypeInfo)}
	b, _ := json.Marshal(r)
	got, err := pmkusum.DecodeRegistration(b)
	if err != nil {
		t.Fatal(err)
	}
	if na, _ := got.NotAfter(); !na.Equal(exp.UTC()) {
		t.Errorf("NotAfter %v, want %v", na, exp)
	}
	for _, bad := range []pmkusum.Registration{
		{IMEI: r.IMEI, Certificate: r.Certificate, InfoTopic: r.InfoTopic},
		{IMEI: r.IMEI, Certificate: r.Certificate, BrokerURL: r.BrokerURL, InfoTopic: pmkusum.Topic("", "S", r.IMEI, pmkusum.TypeData)},
		{IMEI: r.IMEI, Certificate: r.Certificate, BrokerURL: r.BrokerURL, InfoTopic: pmkusum.Topic("", "S", "860000000000002", pmkusum.TypeInfo)},
		{IMEI: r.IMEI, Certificate: "not pem", BrokerURL: r.BrokerURL, InfoTopic: r.InfoTopic},
	} {
		if bad.Check() == nil {
			t.Errorf("%+v was taken", bad)
		}
	}
	if _, err := pmkusum.DecodeRegistration([]byte(`{`)); err == nil {
		t.Error("malformed JSON was taken")
	}
}

// Both OTP interval statements (15/30/60 and 30/60 minutes) are taken;
// others are not.
func TestOTPIntervals(t *testing.T) {
	for _, d := range []time.Duration{15 * time.Minute, 30 * time.Minute, time.Hour, 0} {
		if err := (pmkusum.OTPSource{Secret: []byte("s"), Interval: d}).Check(); err != nil {
			t.Errorf("%v: %v", d, err)
		}
	}
	for _, s := range []pmkusum.OTPSource{{Secret: []byte("s"), Interval: 20 * time.Minute}, {Interval: 30 * time.Minute}, {Secret: []byte("s"), Digits: 4}} {
		if s.Check() == nil {
			t.Errorf("%+v was taken", s)
		}
	}
	if pmkusum.PushIntervals[pmkusum.GroupPump] != 5*time.Minute || pmkusum.PushIntervals[pmkusum.GroupMeter] != 15*time.Minute || pmkusum.PushIntervals[pmkusum.GroupSJB] != 10*time.Minute {
		t.Error("push intervals")
	}
}

// The USPC push carries five channels; all are read.
func TestUSPCFiveChannels(t *testing.T) {
	kw := map[string]any{"PRUNST1": "1"}
	for ch := 1; ch <= 5; ch++ {
		n := string(rune('0' + ch))
		kw["UDKWH"+n], kw["UTOTKWH"+n], kw["UDHR"+n], kw["UTOTHR"+n] = "1.5", "100", "0.5", "50"
		kw["UOPI"+n], kw["UOPV"+n], kw["UOPKW"+n], kw["URUNST"+n], kw["UFLTST"+n] = "2", "230", "0.46", 1, 0
	}
	b, err := pmkusum.Encode(pmkusum.Header{VD: 1, Time: time.Now(), IMEI: "860000000000001", Asset: "ASN_11", AssetValue: "PC1"}, ist, kw)
	if err != nil {
		t.Fatal(err)
	}
	m, err := pmkusum.Decode(b, ist)
	if err != nil {
		t.Fatal(err)
	}
	chans := m.Values["channels"].(map[string]any)
	if len(chans) != 5 || chans["5"].(map[string]any)["outputPower"] != 0.46 || len(m.Other) != 0 {
		t.Errorf("channels %v, other %v", chans, m.Other)
	}
}
