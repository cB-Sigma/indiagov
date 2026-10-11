// Copyright 2026 Abhinav Gautam
// SPDX-License-Identifier: Apache-2.0

package jjm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// The state-to-centre REST API of the report's annexure 9(c)(iii): a
// state server posts each device message to {base}/{Name}DataInsert and
// is answered {"Status", "Message", "DeviceID", "UploadedOn"} with Status
// "1" for success and "0" for failure. Only the flow meter's method
// (FlowmeterDataInsert) is printed; the other names are DeviceType.URLName
// (ours), and DataInsert takes any type.

// CentralSuccess is the success message of the annexure's output sample.
const CentralSuccess = "Data uploaded successfully."

// CentralResponse is the API's answer.
type CentralResponse struct {
	Status     string `json:"Status"`
	Message    string `json:"Message"`
	DeviceID   string `json:"DeviceID"`
	UploadedOn string `json:"UploadedOn"`
}

// OK reports Status "1".
func (r CentralResponse) OK() bool { return strings.TrimSpace(r.Status) == "1" }

// Method is the API method of a device type ("FlowmeterDataInsert");
// "DataInsert" for a type without a name.
func Method(dty string) string {
	if dt, ok := DeviceTypes[strings.ToLower(dty)]; ok {
		return dt.URLName + "DataInsert"
	}
	return "DataInsert"
}

// MethodDTY is the device type a method takes ("" with ok for DataInsert,
// which takes any); ok is false for a name that is no method.
func MethodDTY(method string) (dty string, ok bool) {
	name, found := strings.CutSuffix(method, "DataInsert")
	if !found {
		return "", false
	}
	if name == "" {
		return "", true
	}
	for code, dt := range DeviceTypes {
		if strings.EqualFold(dt.URLName, name) {
			return code, true
		}
	}
	return "", false
}

// PushError is a push that failed. Retry tells whether sending the same
// message again later can succeed (server errors, rate limits, refused
// credentials that may be renewed, Status 0) or not (the request itself
// was refused).
type PushError struct {
	HTTPStatus int
	Message    string
	Retry      bool
}

func (e *PushError) Error() string {
	if e.HTTPStatus != 0 && e.HTTPStatus != http.StatusOK {
		return "the API answered HTTP " + strconv.Itoa(e.HTTPStatus) + ": " + e.Message
	}
	return "the API refused the data: " + e.Message
}

// CentralAuth is how the key is sent: "x-api-key" (default) puts it in
// X-API-Key, "bearer" in Authorization as a bearer token. The annexure
// names API keys without a header (PROVENANCE.md).
type CentralAuth string

// Central posts device messages to a state-to-centre API.
type Central struct {
	BaseURL string
	Key     string
	Auth    CentralAuth
	Client  *http.Client
	// Loc is the time zone dt is written in when a payload has none.
	Loc *time.Location
}

// Post sends one message to its type's method.
func (c Central) Post(ctx context.Context, p Payload) (CentralResponse, error) {
	if p.DT == "" {
		loc := c.Loc
		if loc == nil {
			loc = time.UTC
		}
		p.DT = time.Now().In(loc).Format("2006-01-02T15:04:05")
	}
	body, err := json.Marshal(p)
	if err != nil {
		return CentralResponse{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.BaseURL, "/")+"/"+Method(p.DTY), bytes.NewReader(body))
	if err != nil {
		return CentralResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Key != "" {
		if strings.EqualFold(string(c.Auth), "bearer") {
			req.Header.Set("Authorization", "Bearer "+c.Key)
		} else {
			req.Header.Set("X-API-Key", c.Key)
		}
	}
	client := c.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return CentralResponse{}, &PushError{Message: err.Error(), Retry: true}
	}
	defer resp.Body.Close()
	return readCentral(resp)
}

func readCentral(resp *http.Response) (CentralResponse, error) {
	var out CentralResponse
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	jsonErr := json.Unmarshal(b, &out)
	out.Message, out.UploadedOn = strings.TrimSpace(out.Message), strings.TrimSpace(out.UploadedOn)
	switch {
	case resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests ||
		resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return out, &PushError{HTTPStatus: resp.StatusCode, Message: msgOr(out.Message, resp.Status), Retry: true}
	case resp.StatusCode >= 400:
		return out, &PushError{HTTPStatus: resp.StatusCode, Message: msgOr(out.Message, resp.Status)}
	case jsonErr != nil:
		return out, &PushError{HTTPStatus: resp.StatusCode, Message: "the answer is not JSON: " + jsonErr.Error(), Retry: true}
	case !out.OK():
		return out, &PushError{HTTPStatus: resp.StatusCode, Message: fmt.Sprintf("Status %q: %s", out.Status, out.Message), Retry: true}
	}
	return out, nil
}

func msgOr(m, d string) string {
	if m != "" {
		return m
	}
	return d
}

// ErrRetry reports whether err is worth retrying (a PushError with Retry,
// or a transport error).
func ErrRetry(err error) bool {
	var pe *PushError
	if errors.As(err, &pe) {
		return pe.Retry
	}
	var kr ErrKeyRefused
	return errors.As(err, &kr)
}
