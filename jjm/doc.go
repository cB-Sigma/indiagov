// Copyright 2026 Abhinav Gautam
// SPDX-License-Identifier: Apache-2.0

// Package jjm reads the device-to-cloud messages of the Jal Jeevan
// Mission Technical/Expert Committee report (NJJM, DDWS, March 2021;
// section 5(f) and annexure 9(c)): {"gid", "did", "dty", "dt", "p"} from
// flow meters, ground water and service reservoir level sensors, chlorine,
// turbidity and pH analysers, pressure sensors and pump controllers, with
// the parameter codes, units and error codes of each device type, and the
// gid's state, district, block, village and scheme (ParseGID). It also
// holds the eJalShakti IoT push API (the national dashboard's API: the
// table of APIs, the readers of their JSON and DataContract XML bodies,
// the queued push envelope).
//
// Units and codes are data (model.go, units.go); no text of the report or
// of the portal's manuals is copied.
package jjm
