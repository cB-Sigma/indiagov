# Provenance

## How the code was written

Written for SigmaM2M, a oneM2M platform, in Go, from the public
documents below. No code was ported or copied from another
implementation; none in any language was found with a licence. Topic
layouts, keywords, parameter codes, units, error codes and API member
names are facts every implementation must use; the Go types, the
lenient readers, the builders, the clients and the MQTT seam are our own
design.

The readers and keyword tables were first written inside SigmaM2M's
PM-KUSUM and JJM adapters and moved to this module on 2026-10-06 with
`git mv`. On 2026-10-11, for the first public release, the simulators,
their commands and the tests that start them moved to the separate
module indiagov-sim, and the module gained the writing side of every
message (`pmkusum.Encode`, `Header`, `Keyword`, `AssetKeyword`, the
topic helpers, the annexure 7 command builders, `Command.Answer`,
`Evaluate`, `OTPMessage`, `OTPSource`; `jjm.NewPayload`, `Metadata`,
`Central`, `EJSeriesBody`, `EJDailyBody`, `EJClient`, `ReadEJResponse`)
and its own unit tests. `OTPSource` and the `Evaluate` rules are the
ones SigmaM2M's adapter used, by the same author.

## Sources

| Document | Date | Used for (files) |
|---|---|---|
| "RMS Communication and Security Architecture - PM KUSUM National Portal" (prepared by KPMG; sections 1 to 6 and annexures 1 to 8), published by OREDA as Annexure-E of a tender (also printed as Annexure-F of a SECI RfS); cites the EESL tender's Annexure 8, clauses 4.b to 4.f | 26 September 2022 | topics, header, keyword tables, communication modes and intervals, registration, annexures 1 to 8 including the USPC (pmkusum/keys.go, message.go, topic.go, command.go, encode.go, otp.go, registration.go) |
| "RMS Communication and Security Architecture - PM KUSUM SEDM Platform" (sections 1 to 6 and annexures 1 to 7), published by MAHADISCOM as Annexure-I of a tender; cites the same EESL clauses | 14 July 2020 | the same, without annexure 8 |
| MNRE Office Memorandum F. No. 41/3/2018-SPV Division, updated specification of SPV water pumping systems and the USPC, with its RMS annexure, which leaves the message format to the implementing agency | 22 March 2023 | default 15-minute push and the 1 to 15 minute range (`Interval`); pvPower in `Message.Content` |
| NJJM, DDWS, Ministry of Jal Shakti, Technical/Expert Committee report on measuring and monitoring water service delivery in rural areas (section 5(f), annexures 9(b) and 9(c)) | March 2021 | device types, parameters, units, error codes, ranges, intervals, metadata, the state-to-centre API (jjm/model.go, units.go, encode.go, central.go) |
| NIC/DDWS "Operational Manual - IoT data integration" | 18 February 2026 | the five current eJalShakti APIs, keys, answers (jjm/ejalshakti.go, ejpush.go) |
| The eJalShakti help pages, ejalshakti.gov.in/IoT_API/Help, one per API | read 11 October 2026 | member types, the second body shape, XML root and entry elements |
| The 2021 edition of the operational manual | 2021 | the retired GroundWater and Pressure APIs |
| NIC "IoT Web API Description" (overview) | | the unlisted water quality APIs |
| MQTT 5 (OASIS), through Eclipse Paho | | mqttconn |

SigmaM2M's server names the RMS profile "annexure-h"; the name is kept
there as an identifier for compatibility and does not name a document.
Its findings register cites the 2020 document under that name.

No text of these documents is reproduced.

## Field facts

Two public GitHub repositories without a licence were read on
2026-10-11, for facts only; nothing was copied and the tests use our own
values:

- github.com/Chetan1909/EC200_Display, firmware of a cellular RMS: topics
  written in lower case with a fifth level `sub` (to the device) or
  `pub` (from it), an IoT hub form
  `devices/{IMEI}/messages/events/mtd={type}&ver=iiot-1&app={solution}`
  with cloud-to-device messages under `devicebound`, and an agency
  profile with lower-case keywords. `ParseTopic` and `HubTopic` handle
  the topic forms; `Decode` reads keywords in any case.
- github.com/YashGajjar7017/Api_Fetcher_PYTHON, heartbeat captures of a
  field RMS: POTP and COTP sent as numbers, STINTERVAL with decimals,
  ASN_0 with one digit (`TestDecodeFieldForms`).

## Readings where a document is unclear (F-numbers)

Comments cite these numbers from SigmaM2M's register of document
readings. Each is explained here; the register itself is not part of
this module.

| Number | Document | What is unclear | What this module does |
|---|---|---|---|
| F-470 | RMS 2020 and 2022 samples | the printed samples are not valid JSON (missing commas, typographic quotes, a missing quote) | the printed samples are not used; test messages are our own in the same layouts (indiagov-sim) |
| F-471 | RMS 2020 and 2022, section 3 | the topic table capitalises the solution and message type levels, the sample topic does not | topics are read in any case; written in the sample's lower-case message types |
| F-472 | RMS 2020 and 2022 | section 6's IMEI has 15 digits, the annexure samples 16 | IMEI is taken as text, 15 or 16 digits |
| F-473 | RMS 2020 and 2022, section 6 | DATE is YYYY-MM-DD in section 6 and YYMMDD in the samples; TIMESTAMP has no time zone | both DATE forms are read; TIMESTAMP is read in the caller's time zone |
| F-474 | RMS 2020 and 2022, sections 1, 3, 6 | the OTP message's layout, the OTP's length and generation are not defined; section 3 says every 15, 30 or 60 minutes, section 6 says 30 or 60 | `OTPMessage` and `OTPSource` are ours |
| F-475 | RMS 2020 and 2022, section 3, annexure 7 | no answer topic is given; handshake keywords are upper case in section 6, lower case in annexure 7 | answers go on the command's topic; handshake keywords are read in either case and written in lower case |
| F-476 | RMS 2020 and 2022, sections 3, 4, annexure 7 | remote pump on/off is an on-demand write in the text, a config keyword in the annexure | `Pump` takes the message type |
| F-477 | RMS 2020 and 2022, section 3 | a history push mode is named without its request; LOAD has no values | `Retrieve` is ours |
| F-478 | RMS 2020 and 2022, annexures 1 to 6 | the samples' VD values do not follow the asset index table; two samples have no ASN keyword | the group comes from ASN_nn, else from the keywords |
| F-479 | RMS 2020 and 2022, annexures 1 to 3, 6 | some units are missing or wrong | units ours (kVAh, kVA, kW, A, hours) |
| F-480 | JJM report samples | typographic quotes, a missing quote and a missing comma; leading spaces in the answer sample | the printed samples are not used; test messages are our own (indiagov-sim) |
| F-481 | JJM report | ground water depth is in metres in section 5(f), feet in the annexure | `Decoder.GroundWaterUnit`, default feet, converted to metres |
| F-482 | JJM report | the pump controller lists l without describing it and has no battery | l is read wherever it appears |
| F-483 | JJM report | the pH analyser's IDs repeat the chlorine analyser's; turbidity's type and code differ | did is taken as sent; type tbs with code tb |
| F-484 | JJM report | the flow meter's error codes differ between section 5(f) and the annexure | `Decoder.FlowErrorCodes` |
| F-485 | JJM report | sample values lie outside the report's own reference ranges | values out of range are decoded and flagged range:<code>; our test messages stay in range |
| F-486 | JJM report | the gid should include a scheme, the samples have four parts | four or five parts are split |
| F-487 | JJM report | power and power factor have no code | codes kwh and pf, marked ours |
| F-488 | JJM report section 5(e) | no topic, URL, key header or time zone for device traffic | the decoder takes bytes from any transport; dt is read in the caller's time zone; `Central` sends the key in X-API-Key or as a bearer token |
| F-490 | JJM report annexure 9(c)(iii) | only FlowmeterDataInsert is printed | the other method names are ours (`DeviceType.URLName`) |
| F-2704 | RMS 2022 annexure 8 | USPC keywords end in a channel digit that collides with the instance digit | U keywords are read as channels of the message's controller |
| F-2705 | eJalShakti help pages and manuals | no units; two date forms; member names differ between documents; a wrong key is answered Status true | units configurable; both date forms read; members matched without regard to case or spaces; the not-matched message is a key refusal |
| F-2706 | JJM report and public JJM tenders | no public document gives a device-facing topic, endpoint, register map or command format for JJM controllers | no command format is built in; indiagov-sim's RTU has a shape of our own |
| F-2707 | MNRE and state documents | no other agency publishes its RMS format | only the RMS 2020 and 2022 profile is built in |

Further defects found by an independent reading on 2026-10-11 (no
register number yet):

| Document | What is wrong | What this module does |
|---|---|---|
| RMS 2020 and RMS 2022 | the 2022 version adds annexure 8 (USPC), which the 2020 version lacks | annexure 8 is read in every profile; a 2020 RMS simply never sends it |
| eJalShakti 2026 manual and help pages | GroundWater and Pressure of the 2021 manual are gone; the water quality APIs of the overview are not on the help pages | read; pushed only with `EJClient.AllowRetired` |
| eJalShakti help pages | FlowMeter's VillageId is marked optional, while the manual marks it required | VillageId and GatewayId are required when reading (`EJPayloads`) and always written |
| eJalShakti 2026 manual | a screenshot points the pH and turbidity keys at the LevelSensor URL | not followed: each API gets its own key and URL |
| eJalShakti help pages | the series APIs show a reading at the top level beside the collection; the manuals send the collection only | both read; the collection is written (`EJTopLevel` writes the other) |
| eJalShakti help pages | the pH collection's XML entries are PhLevelinputData, the others are named like their collection | `PortalAPI.XMLItem` per API; any entry name is read |

## Tests

| Test | Source |
|---|---|
| pmkusum and jjm tests, example tests | our own values; round trips of builders and readers, both forms wherever the documents disagree, refusals and faults; field forms with our own values |
| mqttconn/mqttconn_test.go | a fake broker that drops each session |
| indiagov-sim | messages of our own in the documents' layouts, driven through the simulators (same author); the documents' printed samples are not shipped |
