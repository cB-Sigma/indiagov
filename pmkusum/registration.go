// Copyright 2026 Abhinav Gautam
// SPDX-License-Identifier: Apache-2.0

package pmkusum

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"time"
)

// Registration (section 2 of both documents, citing the EESL tender's
// annexure 8 clause 4.d): the state platform issues a client certificate
// for each registered IMEI, and a web API for suppliers returns that
// certificate, the device broker's URL and the info topic; after the
// certificate expires the RMS fetches the renewed one over FTP (the
// FTP keywords of annexure 7). The documents name the items, not a wire
// format, so the JSON names of Registration are ours (the ones SigmaM2M's
// registration API uses).
type Registration struct {
	IMEI     string `json:"imei"`
	Solution string `json:"solution,omitempty"`
	// Certificate is the client certificate (PEM); PrivateKey and
	// CACertificate are sent when the platform makes the key pair.
	Certificate   string `json:"certificate"`
	PrivateKey    string `json:"privateKey,omitempty"`
	CACertificate string `json:"caCertificate,omitempty"`
	BrokerURL     string `json:"brokerUrl"`
	InfoTopic     string `json:"infoTopic"`
	ClientID      string `json:"clientId,omitempty"`
}

// Check reports what a registration answer lacks: a certificate that
// parses, the broker URL and the info topic.
func (r Registration) Check() error {
	if r.IMEI == "" || r.BrokerURL == "" || r.InfoTopic == "" {
		return errors.New("imei, brokerUrl and infoTopic are required")
	}
	if rt, ok := ParseTopic(r.InfoTopic); !ok || rt.Type != TypeInfo || rt.IMEI != r.IMEI {
		return fmt.Errorf("infoTopic %q is not the info topic of %s", r.InfoTopic, r.IMEI)
	}
	_, err := r.NotAfter()
	return err
}

// NotAfter is when the certificate expires (the RMS then fetches the
// renewed one over FTP).
func (r Registration) NotAfter() (time.Time, error) {
	blk, _ := pem.Decode([]byte(r.Certificate))
	if blk == nil {
		return time.Time{}, errors.New("certificate is not PEM")
	}
	c, err := x509.ParseCertificate(blk.Bytes)
	if err != nil {
		return time.Time{}, fmt.Errorf("certificate: %w", err)
	}
	return c.NotAfter, nil
}

// DecodeRegistration reads a registration answer.
func DecodeRegistration(b []byte) (Registration, error) {
	var r Registration
	if err := json.Unmarshal(b, &r); err != nil {
		return r, fmt.Errorf("the registration answer is not JSON: %w", err)
	}
	return r, r.Check()
}

// Push intervals of section 4 (periodic mode, configurable): pump
// controller and inverter every 5 minutes, energy meter every 15, string
// combiner box every 10. Groups without a stated interval have none here.
var PushIntervals = map[Group]time.Duration{
	GroupPump: 5 * time.Minute, GroupInverter: 5 * time.Minute, GroupMeter: 15 * time.Minute, GroupSJB: 10 * time.Minute,
}

// OTPIntervals are the OTP exchange intervals the documents name: 15, 30
// or 60 minutes in the topic table, 30 or 60 minutes in the header table.
// OTPSource.Check takes any of the three.
var OTPIntervals = []time.Duration{15 * time.Minute, 30 * time.Minute, 60 * time.Minute}

// Check reports an OTPSource the documents do not allow: an interval other
// than OTPIntervals, fewer than 6 digits (the shortest the samples print)
// or more than 10, or no secret.
func (s OTPSource) Check() error {
	if len(s.Secret) == 0 {
		return errors.New("an OTP source needs a secret")
	}
	ok := false
	for _, d := range OTPIntervals {
		if s.interval() == d {
			ok = true
		}
	}
	if !ok {
		return fmt.Errorf("the OTP interval %v is not 15, 30 or 60 minutes", s.interval())
	}
	if d := s.digits(); d < 6 || d > 10 {
		return errors.New("an OTP has 6 to 10 digits")
	}
	return nil
}
