// Copyright 2026 Abhinav Gautam
// SPDX-License-Identifier: Apache-2.0

package pmkusum

import (
	"net/url"
	"strings"
)

// Message types: the last topic level of the profile's topic
// table, in the lower case of its sample topic (the table capitalises
// them; F-471 in PROVENANCE.md).
const (
	TypeInfo      = "info"
	TypeOTP       = "otp"
	TypeHeartbeat = "heartbeat"
	TypeData      = "data"
	TypeOnDemand  = "ondemand"
	TypeConfig    = "config"
)

// MessageTypes lists the section 3 message types.
var MessageTypes = []string{TypeInfo, TypeOTP, TypeHeartbeat, TypeData, TypeOnDemand, TypeConfig}

// RMSSubscribes are the message types an RMS receives (the platform
// publishes them) and RMSPublishes those it sends. On-demand and config
// go both ways: the platform's command and the RMS's answer share the
// topic (section 3 names no answer topic; see PROVENANCE.md).
var (
	RMSSubscribes = []string{TypeOTP, TypeInfo, TypeOnDemand, TypeConfig}
	RMSPublishes  = []string{TypeHeartbeat, TypeData, TypeOnDemand, TypeConfig}
)

// TopicPrefix is the first topic level of the profile.
const TopicPrefix = "IIOT-1"

// Solutions are the solution levels of the section 3 topic table.
var Solutions = []string{"Standalonesolarpump", "Gridconnectedsolarpump", "SolarMW", "Ongridrooftop", "Offgridrooftop"}

// Topic builds {prefix}/{solution}/{imei}/{msgType}; an empty prefix is
// TopicPrefix.
func Topic(prefix, solution, imei, msgType string) string {
	if prefix == "" {
		prefix = TopicPrefix
	}
	return prefix + "/" + solution + "/" + imei + "/" + msgType
}

// Route is what a topic names.
type Route struct {
	Prefix, Solution, IMEI string
	// Type is the message type in lower case when it is one of
	// MessageTypes, else as sent.
	Type string
	// Direction is "pub" or "sub" when the topic carries a fifth level of
	// that name (a form RMS firmware in the field uses), else "".
	Direction string
	// Hub is true for the cloud IoT hub form
	// devices/{imei}/messages/events/mtd={type}&ver={prefix}&app={solution}.
	Hub bool
}

// ParseTopic reads an RMS topic. It takes the section 3 form in any
// letter case, the same with a trailing "pub" or "sub" level, and the IoT
// hub form above (both seen in field firmware; PROVENANCE.md). ok is
// false for any other topic.
func ParseTopic(topic string) (Route, bool) {
	lv := strings.Split(topic, "/")
	if len(lv) >= 5 && strings.EqualFold(lv[0], "devices") && strings.EqualFold(lv[2], "messages") {
		return parseHubTopic(lv)
	}
	r := Route{}
	switch len(lv) {
	case 5:
		d := strings.ToLower(lv[4])
		if d != "pub" && d != "sub" {
			return Route{}, false
		}
		r.Direction = d
	case 4:
	default:
		return Route{}, false
	}
	for _, x := range lv[:4] {
		if x == "" {
			return Route{}, false
		}
	}
	r.Prefix, r.Solution, r.IMEI, r.Type = lv[0], lv[1], lv[2], normType(lv[3])
	return r, true
}

func parseHubTopic(lv []string) (Route, bool) {
	r := Route{IMEI: lv[1], Hub: true}
	if r.IMEI == "" {
		return Route{}, false
	}
	switch strings.ToLower(lv[3]) {
	case "devicebound":
		// Cloud-to-device messages: the OTP in the firmware seen.
		r.Type, r.Direction = TypeOTP, "sub"
		return r, true
	case "events":
	default:
		return Route{}, false
	}
	if len(lv) < 5 {
		return Route{}, false
	}
	q, err := url.ParseQuery(strings.Join(lv[4:], "/"))
	if err != nil {
		return Route{}, false
	}
	r.Type, r.Prefix, r.Solution = normType(q.Get("mtd")), q.Get("ver"), q.Get("app")
	if r.Type == "" {
		return Route{}, false
	}
	return r, true
}

// HubTopic builds the IoT hub event topic of a message type.
func HubTopic(prefix, solution, imei, msgType string) string {
	if prefix == "" {
		prefix = TopicPrefix
	}
	return "devices/" + imei + "/messages/events/mtd=" + msgType + "&ver=" + strings.ToLower(prefix) + "&app=" + strings.ToLower(solution)
}

func normType(s string) string {
	l := strings.ToLower(s)
	for _, t := range MessageTypes {
		if l == t {
			return t
		}
	}
	return s
}
