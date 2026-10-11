# indiagov

Device messages of two Indian government schemes in pure Go: the PM-KUSUM solar pump Remote Monitoring System (RMS) message profile, and Jal Jeevan Mission device data with the state-to-centre and eJalShakti push APIs; not affiliated with any government body.

[![Go Reference](https://pkg.go.dev/badge/github.com/cB-Sigma/indiagov.svg)](https://pkg.go.dev/github.com/cB-Sigma/indiagov) [![test](https://github.com/cB-Sigma/indiagov/actions/workflows/test.yml/badge.svg)](https://github.com/cB-Sigma/indiagov/actions/workflows/test.yml) [![Licence: Apache-2.0](https://img.shields.io/badge/licence-Apache--2.0-blue.svg)](LICENSE) [![Go](https://img.shields.io/badge/go-1.24.0-00ADD8.svg)](go.mod)

`pmkusum` reads and writes the messages of "RMS Communication and
Security Architecture", the RMS profile PM-KUSUM agencies publish with
their tenders: MQTT topics IIOT-1/{solution}/{IMEI}/{type}, the message
header, the equipment keywords of annexures 1 to 6, the on-demand and
config exchanges of annexure 7 and the five USPC channels of annexure 8,
with one-time passwords, registration answers and the topic forms seen
in field firmware. `jjm` reads and writes the device-to-cloud messages
of the Jal Jeevan Mission Technical/Expert Committee report (eight
device types, units, both error code tables), posts them to a
state-to-centre API, and writes, reads and pushes the bodies of the
eJalShakti IoT APIs in JSON and DataContract XML. `mqttconn` is a small
MQTT 5 seam on Eclipse Paho. No cgo. It was written for SigmaM2M, a
oneM2M platform, whose PM-KUSUM and JJM adapters are built on it.

## Documents followed

| Short name here | Document | Date | Where it is published |
|---|---|---|---|
| RMS 2020 | "RMS Communication and Security Architecture - PM KUSUM SEDM Platform" (annexures 1 to 7) | 14 July 2020 | MAHADISCOM, as Annexure-H of a tender (2021 upload; the copy we hashed) and as Annexure-I (2022 upload) |
| RMS 2022 | "RMS Communication and Security Architecture - PM KUSUM National Portal" (prepared by KPMG; annexures 1 to 8) | 26 September 2022 | OREDA, as Annexure-E of a tender |
| | both cite the EESL tender's Annexure 8, clauses 4.b to 4.f | | |
| JJM report | NJJM, DDWS, Technical/Expert Committee report on measuring and monitoring water service delivery in rural areas | March 2021 | jaljeevanmission.gov.in |
| eJalShakti 2026 | NIC/DDWS "Operational Manual - IoT data integration" and the live help pages at ejalshakti.gov.in/IoT_API/Help | 18 February 2026; pages read 11 October 2026 | ejalshakti.gov.in |
| eJalShakti 2021 | the earlier edition of the same manual | 2021 | ejalshakti.gov.in |

None of these is an MNRE specification of the message format: MNRE's
RMS specification leaves the format to the implementing agency.
PROVENANCE.md has the details. SigmaM2M calls the RMS profile
"annexure-h": an identifier there, taken from the label of the 2021
MAHADISCOM copy of the 2020 document.

## Support matrix

Every message or operation the documents name. "Ours" marks a choice
where a document is silent; PROVENANCE.md explains each.

### PM-KUSUM RMS (RMS 2020 and RMS 2022)

| Message or operation | Status |
|---|---|
| Topics: info, otp, heartbeat, data, ondemand, config under IIOT-1/{solution}/{IMEI}; the five solution levels | yes: `Topic`, `ParseTopic` (any letter case), `Solutions`, `RMSSubscribes`, `RMSPublishes` |
| Topic forms of field firmware: lower case with a pub or sub fifth level; IoT hub event topics | yes: `ParseTopic`, `HubTopic` (no document has them) |
| Header: VD, TIMESTAMP, MAXINDEX, INDEX, LOAD, STINTERVAL, MSGID, COMMAND, DATE, IMEI, POTP, COTP | yes: `Decode`, `Encode` (`Header`); keywords in any case |
| DATE as YYYY-MM-DD (header table) and as the number YYMMDD (every annexure sample) | both read (`ParseStorageDate`); YYMMDD written, as the samples do |
| Asset index ASN_nn (0 RMS, 1-9 DAQ, 11-19 pump controller, 21-29 meter, 31-39 inverter, 41-49 string combiner box) | yes: `Decode`, `AssetKeyword` |
| Annexure 1, pump controller (revision B), run status 0 to 6, control mode 0 to 2 | yes: decode, encode (`Keyword`), `RunStatus`, `ControlMode` |
| Annexures 2 to 6: energy meter, inverter, string combiner box (24 channels), RMS heartbeat, DAQ | yes |
| Annexure 8 (RMS 2022 only): USPC channels 1 to 5 (UDKWH, UTOTKWH, UDHR, UTOTHR, UOPI, UOPV, UOPKW, URUNST, UFLTST) | yes: read into `Values["channels"]`; written with `Encode` |
| Annexure 7: on-demand and config read and write, handshake keywords returned as received, an unknown keyword answered with 0 | yes: `Read`, `Write`, `Command.Encode`, `DecodeCommand`, `Command.Answer`, `Evaluate` (`Result.Invalid`) |
| Annexure 7 keywords: APN, user and password per SIM slot, restart, push and heartbeat intervals, RTC date and time, GSM time sync, DO1 (pump), analog input zero and span, MQTT server, FTP download | yes: `APN`, `Restart`, `Interval`, `HeartbeatInterval`, `SyncClock`, `GSMSync`, `Pump`, `AnalogCalibration`, `Broker`, `Firmware`; credentials masked (`MaskSecrets`) |
| A write the controller does not apply (local mode, protection trip) | yes: `Result.NotApplied` |
| Communication modes: periodic push (pump and inverter every 5 minutes, meter 15, string box 10), push on event, on-demand, config | yes: `PushIntervals`; events are ordinary data messages |
| History recovery by INDEX and LOAD (the mode is named, its request is not) | ours: `Retrieve` |
| OTP pair on the otp topic, every 15, 30 or 60 minutes (topic section) or 30 or 60 (header table) | both statements taken (`OTPIntervals`, `OTPSource.Check`); the otp message's layout and how OTPs are made are not defined, so `OTPMessage` and `OTPSource` are ours |
| Registration: a web API returns the client certificate, broker URL and info topic; certificates renewed over FTP | the answer's items: `Registration`, `DecodeRegistration` (JSON names ours; no wire format is published); renewal uses the annexure 7 FTP keywords (`Firmware`) |
| info message | carried as bytes: its content is not defined |
| MNRE RMS items without a keyword: pumping head, monthly, yearly and five-year energy | Not yet: no keyword exists; pvPower is derived from DC voltage and current in `Message.Content` |
| Other agencies' formats; vendor keywords seen in firmware (not in any document) | Not yet: none is published; unknown keywords are kept in `Message.Other` |

### Jal Jeevan Mission (JJM report)

| Message or operation | Status |
|---|---|
| Device message {gid, did, dty, dt, p}, section 5(f) and annexure 9(c)(i) | yes: `Decoder`, `NewPayload`, `Payload.JSON`; members in any case |
| Device types fm, gl, sl, ca, tbs, ph, ps, pc with their codes, battery b, error code e, empty p4 to p6 slots | yes: `DeviceTypes`, `Params` |
| Codes named without a device type or code: conductivity cd; pump power (kWh) and power factor; the generic level l | cd and l read; kwh and pf are our codes |
| Flow meter error codes: annexure table (01 tamper, 02 other) and section 5(f) table (01 leakage, 02 tamper, 03 other); 11 end of day | both: `Decoder.FlowErrorCodes`, default the annexure's; `Reading.EndOfDay` |
| Ground water depth in feet (annexure) or metres (section 5(f)) | both: `Decoder.GroundWaterUnit`, default feet, recorded in metres |
| Collection and transmission intervals per type and power source | yes: `DeviceType.Battery`, `DeviceType.Mains`, `Interval` |
| gid parts: state, district, block, village, scheme | yes: `ParseGID` |
| Deployment metadata, annexure 9(c)(ii) | yes: `Metadata` (the report lists items; the field names are ours) |
| State-to-centre API, annexure 9(c)(iii): FlowmeterDataInsert, answer {Status "1"/"0", Message, DeviceID, UploadedOn} | yes: `Central.Post`, `CentralResponse`, `Method`, `MethodDTY`; the other types' method names are ours |
| Reference ranges and quality flags | ours: `Reading.Flags`, `Reading.Quality` (the report gives ranges, not flags) |
| Transports named in section 5(e): MQTT, HTTPS, AMQP, CoAP | the decoder takes bytes from any transport; MQTT through `mqttconn`; no AMQP or CoAP client here (Not yet) |
| Commands to devices (reporting frequency over the air) | Not yet: no format is published |
| Alerts and KPIs, section 3(b) | Not yet: application logic, not a message |

### eJalShakti IoT push API

| Message or operation | Status |
|---|---|
| FlowMeter: WaterSupply, LPCD, Date (one value per village and day) | yes: `EJDailyBody`, `EJPayloads`, `EJClient.Push` |
| Chlorine, PHLevel, TurbidityLevel, LevelSensor: readings with TimeStamp in the collection (ChlorineData, PhLevelData, TurbidityLevelData, LevelSensorData) | yes: `EJSeriesBody` |
| The help pages' other shape: a reading at the top level beside the collection | read; written with `EJTopLevel` |
| JSON bodies | written and read; member names in any case and with stray spaces |
| DataContract XML bodies (roots Flowmeterinput, Chlorineinput, PhLevelinput, TurbidityLevelinput, LevelSensorinput; pH entries PhLevelinputData) | written (`EJXML`, `EJClient.XML`) and read |
| Date as YYYY-MM-DD (manual) or a full date-time (help page); TimeStamp with seven fractional digits and +05:30 | both read; `EJTimeLayout` written |
| Key per API in Authorization, as it is (no scheme); keys expire and are renewed on the portal | yes: `EJClient.Keys`; a refused key is `ErrKeyRefused` |
| Answer {Status, Message}: Status true with "sucess", and Status true with "KeyToken not Matched" for a wrong key | yes: `ReadEJResponse` |
| GroundWater (Depth, Date) and Pressure (PressureSensorData) of the 2021 manual, gone from the 2026 manual and the help pages | read; pushed only with `EJClient.AllowRetired` (`PortalAPI.Retired`) |
| Nitrate, Fluoride, Iron, Salinity, Arsenic, TDS (named in the "IoT Web API Description" overview, not on the help pages) | read; pushed only with `AllowRetired` (`PortalAPI.Unlisted`) |
| A queued push | yes: `CentralEnvelope`, `QueuedAPI`, `EJClient.PushQueued` |
| Sensor registration and key generation on the portal | outside the API (the portal's web pages) |

## Document defects

The documents contain mistakes and contradictions. This module does not
copy them into behaviour; each is listed in PROVENANCE.md with what the
module does instead. In short: the RMS samples are not valid JSON as
printed, some units are wrong (KVAHIMP and KVAHEXP described in kWh,
PMDKVAEXP described as an import, "DC I" and "DC KW"), the samples' VD
values do not follow the asset index table, the two DATE forms and the
two OTP interval statements disagree; in the JJM report the pH device ID
example repeats the chlorine analyser's, sample values lie outside the
report's own ranges, the flow meter error codes and the ground water
unit differ between section 5(f) and the annexure; in the eJalShakti
material WaterSupply is an integer in the manual and a decimal on the
help page, FlowMeter's VillageId is optional on the help page and
required in the manual, the pH sample has a key with a leading space,
and a screenshot points the pH and turbidity keys at the LevelSensor URL.

## Choosing a library

No other open-source implementation of either profile was found (a
search of GitHub code and repositories on 2026-10-11, cut short by
GitHub's rate limit, for the profiles' keywords, topics and API names).
Two public repositories without a licence show the RMS profile in the
field: RMS firmware and heartbeat captures. They were read for facts
only, nothing was copied, and the forms they show are handled (see
Interop). Pick this module for a Go program that receives, sends or
pushes these messages; it brings no server, database or device model of
its own.

## Install

```
go get github.com/cB-Sigma/indiagov@latest
```

Requires Go 1.24.0 or later.

## Quick start

```go
import (
	"github.com/cB-Sigma/indiagov/jjm"
	"github.com/cB-Sigma/indiagov/pmkusum"
)

ist := time.FixedZone("IST", 19800)

// PM-KUSUM: a message from the broker, then a command and its answer.
rt, _ := pmkusum.ParseTopic(topic)              // IIOT-1/Standalonesolarpump/<IMEI>/data
m, err := pmkusum.Decode(payload, ist)          // m.Group, m.Instance, m.Values["outputPower"]
cmd, _ := pmkusum.Interval("41", time.Now(), 10)
b, _ := cmd.Encode(ist)                         // publish on pmkusum.Topic("", rt.Solution, rt.IMEI, cmd.Type)
// ... the answer with msgid 41 arrives on the same topic
res := pmkusum.Evaluate(cmd, answer)            // res.Invalid, res.NotApplied, res.Err()

// JJM: a device message, a push to the centre and to eJalShakti.
r, err := jjm.Decoder{Loc: ist}.Decode(msg)     // r.Values, r.Quality()
_, err = jjm.Central{BaseURL: centralURL, Key: key}.Post(ctx, r.Source)
api, _ := jjm.EjalshaktiAPIOf("ca")
body, _ := jjm.EJSeriesBody(api, villageID, gatewayID, ist, jjm.EJReading{Time: r.Time, Value: 0.4})
_, err = jjm.EJClient{Keys: keys}.Push(ctx, api.Name, body) // jjm.ErrKeyRefused: renew the key
```

The runnable examples are `ExampleDecode` and `ExampleDecoder`.
[indiagov-sim](https://github.com/cB-Sigma/indiagov-sim) runs these
flows against simulated RMS units, gateways, a central API and a
portal.

## Documentation

API reference: https://pkg.go.dev/github.com/cB-Sigma/indiagov

| Package | What it does |
|---|---|
| `pmkusum` | `Decode`, `Message` (`Content`, `Container`), `Encode`, `Header`, `Keyword`, `AssetKeyword`, `Params`; topics (`Topic`, `ParseTopic`, `HubTopic`); commands (`Read`, `Write`, the action builders, `Command.Encode`, `DecodeCommand`, `Command.Answer`, `Evaluate`, `Result`); OTPs (`OTPMessage`, `DecodeOTP`, `OTPSource`, `OTPIntervals`); `Registration`; `PushIntervals`; keyword tables (`KnownKey`, `ConfigKeys`, `SecretKeys`, `RunStatus`, `ControlMode`) |
| `jjm` | `Decoder`, `Reading`, `Payload`, `NewPayload`, `ParseGID`, `DeviceTypes`, `Params`, `CheckUnit`, `Interval`, `Metadata`; the state-to-centre API (`Central`, `CentralResponse`, `Method`, `MethodDTY`); eJalShakti (`PortalAPIs`, `EjalshaktiAPI`, `EJPayloads`, `EJSeriesBody`, `EJDailyBody`, `EJTopLevel`, `EJXML`, `EJClient`, `ReadEJResponse`, `ErrKeyRefused`, `CentralEnvelope`); `PushError`, `ErrRetry` |
| `mqttconn` | `Dialer`, `Conn`, `Options`, and `Paho`, an MQTT 5 dialer on Eclipse Paho with paced reconnection |

Changes per version are in [CHANGELOG.md](CHANGELOG.md).

## Testing

```
go vet ./...
go test -race ./...
```

The unit tests use our own values: round trips of every builder and
reader, both forms wherever the documents disagree, the refusals and
faults each reader and client must handle (malformed bodies, error
codes, ranges, refused keys, Status 0, server errors, unreachable
servers, retired APIs), and the field forms below. The tests that drive
the simulators are in indiagov-sim, so this module does not depend on
its simulators. Tests may carry `// Covers: SIG-...` comments:
requirement IDs of the SigmaM2M project's compliance report, with no
meaning outside it. Code comments that cite `F-` numbers refer to
SigmaM2M's register of readings where a document is unclear or
contradicts itself; PROVENANCE.md explains each one.

### Interop tested against

No independent implementation of either profile exists to test
against: none was found on GitHub, and neither an agency platform, the
eJalShakti portal nor a field controller has been run against this
module. What was used instead:

- Messages of our own built to the documents' layouts (every annexure 1
  to 8 layout, the JJM report's eight device layouts and its API
  answer, the eJalShakti bodies of the help pages in JSON and XML), in
  indiagov-sim. The documents' own printed samples are not shipped.
- The eJalShakti help pages' model descriptions (read 2026-10-11) for
  member names, XML root and entry elements and the two body shapes.
- Facts from two public GitHub repositories without a licence, read
  only, nothing copied: RMS firmware for a cellular module, which
  writes lower-case topics with a trailing pub or sub level, an IoT
  hub topic form and lower-case keywords; and heartbeat captures from
  a field RMS, which carry OTPs as numbers, STINTERVAL with decimals
  and ASN_0. Each form is decoded in this module's tests with our own
  values.
- The simulators in indiagov-sim. They and this module have the same
  author and were written from the same documents, so their agreement
  is not independent evidence.

## Not yet

| What | Why |
|---|---|
| A run against an agency platform, the eJalShakti portal or a field RMS | needs access to them; not available |
| A registration request format | no document publishes one; `Registration` holds the answer's items |
| Device commands for JJM controllers | no format is published |
| AMQP and CoAP clients | the decoder takes bytes from any transport; bring your own client |
| Pumping head and period energy keywords | the documents have none |

## Demo values

Every IMEI, gid, device ID, village ID, gateway ID, key, password and
host name in this module is a made-up demo value.

## Related repositories

- [indiagov-sim](https://github.com/cB-Sigma/indiagov-sim): simulated
  RMS units, JJM gateways and RTUs, a central API and an eJalShakti
  portal, with fault switches, as Go packages and commands.

## Status

v0.x: the API may change before v1.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## Security

See [SECURITY.md](SECURITY.md) to report a vulnerability privately.

## Licence

Apache-2.0 (see [LICENSE](LICENSE) and [NOTICE](NOTICE)). Copyright 2026
Abhinav Gautam. PM-KUSUM, Jal Jeevan Mission and eJalShakti are names of
Government of India schemes and services, used only to say which
messages this module handles; this module is not made, endorsed or
supported by any ministry, agency, utility or company.
