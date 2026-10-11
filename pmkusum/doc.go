// Copyright 2026 Abhinav Gautam
// SPDX-License-Identifier: Apache-2.0

// Package pmkusum reads and writes the messages of PM-KUSUM solar water
// pump Remote Monitoring Systems (RMS) in the profile of "RMS
// Communication and Security Architecture": the PM KUSUM SEDM Platform
// version (14/07/2020, published by MAHADISCOM as Annexure-H in a 2021
// upload and as Annexure-I in a 2022 upload) and the PM KUSUM National
// Portal version (26/09/2022, published by OREDA as Annexure-E), both
// citing the EESL tender's Annexure 8, clause 4. In this module "the
// profile" means that message structure; SigmaM2M's profile identifier
// "annexure-h" comes from the label of the 2021 MAHADISCOM copy.
//
// Covered: MQTT topics IIOT-1/{solution}/{IMEI}/{info|otp|heartbeat|data|
// ondemand|config} (with the forms field firmware uses), the header
// (VD, TIMESTAMP, INDEX, MAXINDEX, LOAD, STINTERVAL, MSGID, COMMAND, DATE,
// IMEI, POTP, COTP), the equipment keywords of annexures 1 to 6 (pump
// controller, energy meter, inverter, string combiner box, RMS heartbeat,
// DAQ), the on-demand and configuration messages of annexure 7, the five
// USPC channels of annexure 8 (2022 version), OTPs and registration.
//
// Decode is lenient where that is safe: keywords in any case, both DATE
// forms, TIMESTAMP variants, numbers sent as text, and keywords no table
// knows (kept in Message.Other). The keyword tables are data (keys.go); no
// text of the documents is copied.
package pmkusum
