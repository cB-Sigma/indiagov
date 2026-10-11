# Changelog

All notable changes to this module. Versions follow semantic versioning;
before v1 the API may change in minor versions.

## v0.1.0 (2026-10-11)

First release.

- `pmkusum`: the RMS message profile of "RMS Communication and Security
  Architecture" (PM KUSUM SEDM Platform, 2020; PM KUSUM National
  Portal, 2022), read (`Decode`) and written (`Encode`, `Header`,
  `Keyword`, `AssetKeyword`): annexures 1 to 7 and the five USPC
  channels of annexure 8; topics (`Topic`, `ParseTopic`, `HubTopic`)
  including the pub/sub and IoT hub forms of field firmware; annexure 7
  commands (`Read`, `Write`, action builders, `Command.Encode`,
  `DecodeCommand`, `Command.Answer`, `Evaluate`); OTP messages, an OTP
  source and both OTP interval statements; registration answers;
  push intervals.
- `jjm`: the JJM report's device messages, read (`Decoder`, both flow
  error code tables, both ground water units) and written
  (`NewPayload`); deployment `Metadata`; the state-to-centre API client
  (`Central`); the eJalShakti API table (current, retired and unlisted
  APIs), body readers (JSON and XML, both body shapes), body writers
  (`EJSeriesBody`, `EJDailyBody`, `EJTopLevel`, `EJXML`), answer
  classification and client (`EJClient`).
- `mqttconn`: an MQTT 5 seam with an Eclipse Paho dialer.
- The simulators and their commands are in the separate module
  indiagov-sim.
