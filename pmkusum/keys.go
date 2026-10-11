package pmkusum

import (
	"strings"
)

// Keyword tables of the RMS message profile of "RMS Communication and
// Security Architecture" (the PM KUSUM SEDM Platform version of 14/07/2020,
// published by MAHADISCOM as Annexure-H (2021) and Annexure-I (2022), and the PM KUSUM National Portal
// version of 26/09/2022, published by OREDA as Annexure-E; both cite the
// EESL tender's Annexure 8, clause 4): the message structure's header
// keywords and annexures 1 to 7 (annexure 8, the USPC, is uspcTable below).
// Each equipment keyword ends in
// the instance number of the asset (1 to 9); the tables hold the keyword
// without it. Names and units are ours; the source gives a description
// and, for the pump controller, a unit column.

// Group is an equipment class of the asset index table printed in every
// annexure (RMS 0, DAQ 1-9, pump controller 11-19, meter 21-29, inverter
// 31-39, string combiner box 41-49).
type Group string

// Groups.
const (
	GroupRMS      Group = "rms"
	GroupDAQ      Group = "daq"
	GroupPump     Group = "pump"
	GroupMeter    Group = "meter"
	GroupInverter Group = "inverter"
	GroupSJB      Group = "sjb"
)

// groupOfASN maps the tens digit of an ASN_nn asset index to its group
// (annexures 1 to 4 asset index table).
func groupOfASN(n int) (Group, int, bool) {
	switch {
	case n == 0:
		return GroupRMS, 0, true
	case n >= 1 && n <= 9:
		return GroupDAQ, n, true
	case n >= 11 && n <= 19:
		return GroupPump, n - 10, true
	case n >= 21 && n <= 29:
		return GroupMeter, n - 20, true
	case n >= 31 && n <= 39:
		return GroupInverter, n - 30, true
	case n >= 41 && n <= 49:
		return GroupSJB, n - 40, true
	}
	return "", 0, false
}

// Param is one keyword of an equipment table.
type Param struct {
	Name string
	Unit string
	// Text keeps the value as text (serial numbers, codes).
	Text bool
}

// headerKeys are the section 6 keywords common to every message; they are
// matched without an instance suffix.
var headerKeys = map[string]bool{
	"VD": true, "TIMESTAMP": true, "MAXINDEX": true, "INDEX": true, "LOAD": true, "STINTERVAL": true,
	"MSGID": true, "DATE": true, "IMEI": true, "POTP": true, "COTP": true, "COMMAND": true,
}

// Tables by group (annexure 1 pump controller revision B, 2 energy meter,
// 3 inverter, 4 string combiner box, 6 DAQ).
var tables = map[Group]map[string]Param{
	GroupPump: {
		"PMAXFREQ":  {Name: "maxFrequency", Unit: "Hz"},
		"PFREQLSP":  {Name: "lowerFrequencyLimit", Unit: "Hz"},
		"PFREQHSP":  {Name: "upperFrequencyLimit", Unit: "Hz"},
		"PCNTRMODE": {Name: "controlMode"},
		"PRUNST":    {Name: "runStatus"},
		"PREFFREQ":  {Name: "referenceFrequency", Unit: "Hz"},
		"POPFREQ":   {Name: "outputFrequency", Unit: "Hz"},
		"POPI":      {Name: "outputCurrent", Unit: "A"},
		"POPV":      {Name: "outputVoltage", Unit: "V"},
		"POPKW":     {Name: "outputPower", Unit: "kW"},
		"PDC1V":     {Name: "dcVoltage", Unit: "V"},
		"PDC1I":     {Name: "dcCurrent", Unit: "A"},
		"PDCVOC":    {Name: "dcOpenCircuitVoltage", Unit: "V"},
		"PDKWH":     {Name: "todayEnergy", Unit: "kWh"},
		"PTOTKWH":   {Name: "totalEnergy", Unit: "kWh"},
		"POPFLW":    {Name: "flowRate", Unit: "L/min"},
		"POPDWD":    {Name: "todayWater", Unit: "L"},
		"POPTOTWD":  {Name: "totalWater", Unit: "L"},
		"PMAXDCV":   {Name: "maxDcVoltage", Unit: "V"},
		"PMAXDCI":   {Name: "maxDcCurrent", Unit: "A"},
		"PMAXKW":    {Name: "maxOutputPower", Unit: "kW"},
		"PMAXFLW":   {Name: "maxFlowRate", Unit: "L/min"},
		"PDHR":      {Name: "todayRunHours", Unit: "h"},
		"PTOTHR":    {Name: "totalRunHours", Unit: "h"},
	},
	GroupMeter: {
		"MTDET":      {Name: "meterDetail", Text: true},
		"MTBLDATE":   {Name: "billingDay"},
		"DATE":       {Name: "meterDate", Text: true},
		"TIME":       {Name: "meterTime", Text: true},
		"IR":         {Name: "currentR", Unit: "A"},
		"IY":         {Name: "currentY", Unit: "A"},
		"IB":         {Name: "currentB", Unit: "A"},
		"VRN":        {Name: "voltageRN", Unit: "V"},
		"VYN":        {Name: "voltageYN", Unit: "V"},
		"VBN":        {Name: "voltageBN", Unit: "V"},
		"VRY":        {Name: "voltageRY", Unit: "V"},
		"VYB":        {Name: "voltageYB", Unit: "V"},
		"VBR":        {Name: "voltageBR", Unit: "V"},
		"PFR":        {Name: "powerFactorR"},
		"PFY":        {Name: "powerFactorY"},
		"PFB":        {Name: "powerFactorB"},
		"FRQ":        {Name: "frequency", Unit: "Hz"},
		"POWR":       {Name: "activePowerR", Unit: "kW"},
		"POWY":       {Name: "activePowerY", Unit: "kW"},
		"POWB":       {Name: "activePowerB", Unit: "kW"},
		"POW":        {Name: "activePower", Unit: "kW"},
		"RPOWR":      {Name: "reactivePowerR", Unit: "kvar"},
		"RPOWY":      {Name: "reactivePowerY", Unit: "kvar"},
		"RPOWB":      {Name: "reactivePowerB", Unit: "kvar"},
		"RPOW":       {Name: "reactivePower", Unit: "kvar"},
		"APOWR":      {Name: "apparentPowerR", Unit: "kVA"},
		"APOWY":      {Name: "apparentPowerY", Unit: "kVA"},
		"APOWB":      {Name: "apparentPowerB", Unit: "kVA"},
		"APOW":       {Name: "apparentPower", Unit: "kVA"},
		"KWHNET":     {Name: "netEnergy", Unit: "kWh"},
		"KWHIMP":     {Name: "importEnergy", Unit: "kWh"},
		"KWHEXP":     {Name: "exportEnergy", Unit: "kWh"},
		"KVAHNET":    {Name: "netApparentEnergy", Unit: "kVAh"},
		"KVAHIMP":    {Name: "importApparentEnergy", Unit: "kVAh"},
		"KVAHEXP":    {Name: "exportApparentEnergy", Unit: "kVAh"},
		"MDKWIMP":    {Name: "risingDemandImport", Unit: "kW"},
		"MDKWEXP":    {Name: "risingDemandExport", Unit: "kW"},
		"POFF":       {Name: "powerFailureMinutes", Unit: "min"},
		"TC":         {Name: "tamperCount"},
		"PF":         {Name: "averagePowerFactor"},
		"LBKWHNET":   {Name: "lastBillingNetEnergy", Unit: "kWh"},
		"LBKWHIMP":   {Name: "lastBillingImportEnergy", Unit: "kWh"},
		"LBKWHEXP":   {Name: "lastBillingExportEnergy", Unit: "kWh"},
		"PMDKVAIMP":  {Name: "maxDemandImportKva", Unit: "kVA"},
		"PMDKVAEXP":  {Name: "maxDemandExportKva", Unit: "kVA"},
		"LBMDKWIMP":  {Name: "lastBillingMaxDemandImport", Unit: "kW"},
		"LBMDKWEXP":  {Name: "lastBillingMaxDemandExport", Unit: "kW"},
		"LBMDKVAIMP": {Name: "lastBillingMaxDemandImportKva", Unit: "kVA"},
		"LBMDKVAEXP": {Name: "lastBillingMaxDemandExportKva", Unit: "kVA"},
		"MDRSTC":     {Name: "maxDemandResetCount"},
	},
	GroupInverter: {
		"IST":    {Name: "status"},
		"IFREQ":  {Name: "frequency", Unit: "Hz"},
		"IPF":    {Name: "powerFactor"},
		"IRPHV":  {Name: "voltageR", Unit: "V"},
		"IRPHI":  {Name: "currentR", Unit: "A"},
		"IRPHKW": {Name: "activePowerR", Unit: "kW"},
		"IYPHV":  {Name: "voltageY", Unit: "V"},
		"IYPHI":  {Name: "currentY", Unit: "A"},
		"IYPHKW": {Name: "activePowerY", Unit: "kW"},
		"IBPHV":  {Name: "voltageB", Unit: "V"},
		"IBPHI":  {Name: "currentB", Unit: "A"},
		"IBPHKW": {Name: "activePowerB", Unit: "kW"},
		"IKW":    {Name: "activePower", Unit: "kW"},
		"ITKWH":  {Name: "todayEnergy", Unit: "kWh"},
		"ITON":   {Name: "todayOnTime", Unit: "h"},
		"ILKWH":  {Name: "lifetimeEnergy", Unit: "kWh"},
		"ILON":   {Name: "lifetimeRunHours", Unit: "h"},
		"ITEMP":  {Name: "temperature", Unit: "Cel"},
		"IKVA":   {Name: "apparentPower", Unit: "kVA"},
		"IKVAR":  {Name: "reactivePower", Unit: "kvar"},
	},
	GroupSJB: {
		"SDCV":     {Name: "dcVoltage", Unit: "V"},
		"SDCTOTI":  {Name: "totalDcCurrent", Unit: "A"},
		"SDCTOTKW": {Name: "totalDcPower", Unit: "kW"},
	},
	GroupDAQ: {},
	GroupRMS: {
		"GSM":        {Name: "gsm"},
		"SIM":        {Name: "simDetected"},
		"NET":        {Name: "inNetwork"},
		"GPRS":       {Name: "gprs"},
		"RSSI":       {Name: "rssi"},
		"SD":         {Name: "sdCard"},
		"ONLINE":     {Name: "online"},
		"GPS":        {Name: "gpsModule"},
		"GPSLOC":     {Name: "gpsLocked"},
		"RF":         {Name: "rfModule"},
		"RTCDATE":    {Name: "rtcDate", Text: true},
		"RTCTIME":    {Name: "rtcTime", Text: true},
		"TEMP":       {Name: "temperature", Unit: "Cel"},
		"LAT":        {Name: "latitude", Unit: "deg"},
		"LONG":       {Name: "longitude", Unit: "deg"},
		"SIMSLOT":    {Name: "simSlot"},
		"SIMCHNGCNT": {Name: "simSlotChanges"},
		"FLASH":      {Name: "flash"},
		"BATTST":     {Name: "onBattery"},
		"VBATT":      {Name: "batteryVoltage", Unit: "V"},
		"PST":        {Name: "powerSupply"},
	},
}

func init() {
	inv := tables[GroupInverter]
	for i := 1; i <= 4; i++ {
		n := string(rune('0' + i))
		inv["IDC"+n+"V"] = Param{Name: "dc" + n + "Voltage", Unit: "V"}
		inv["IDC"+n+"I"] = Param{Name: "dc" + n + "Current", Unit: "A"}
		inv["IDC"+n+"KW"] = Param{Name: "dc" + n + "Power", Unit: "kW"}
	}
	for i := 1; i <= 5; i++ {
		n := string(rune('0' + i))
		inv["IFT"+n] = Param{Name: "fault" + n}
	}
	sjb := tables[GroupSJB]
	for i := 1; i <= 24; i++ {
		n := itoa(i)
		sjb["SI"+n] = Param{Name: "channel" + n + "Current", Unit: "A"}
	}
	sjb["SDI1"] = Param{Name: "digitalInput1"}
	sjb["SDI2"] = Param{Name: "digitalInput2"}
	for i := 1; i <= 3; i++ {
		n := itoa(i)
		sjb["ST"+n] = Param{Name: "temperature" + n, Unit: "Cel"}
	}
	daq := tables[GroupDAQ]
	for i := 1; i <= 4; i++ {
		n := itoa(i)
		daq["AI"+n] = Param{Name: "analogInput" + n}
		daq["DI"+n] = Param{Name: "digitalInput" + n}
		daq["DO"+n] = Param{Name: "digitalOutput" + n}
	}
}

// ConfigKeys are the configuration keywords of annexure 7 (on demand and
// config read/write), used to tell an invalid key the RMS answered with 0
// from a known one.
var ConfigKeys = map[string]bool{
	"APN1": true, "USR1": true, "PASS1": true, "APN2": true, "USR2": true, "PASS2": true,
	"RESTART": true, "UPDATEINTERVAL": true, "HEARTINTERVAL": true, "URTCDATE": true, "URTCTIME": true,
	"UPDATERTC": true, "GSMSYNC": true, "DO1": true,
	"AI1ZERO": true, "AI1SPAN": true, "AI2ZERO": true, "AI2SPAN": true,
	"AI3ZERO": true, "AI3SPAN": true, "AI4ZERO": true, "AI4SPAN": true,
	"URL": true, "PORT": true, "CID": true, "USERNAME": true, "PASSWORD": true,
	"FTPURL": true, "FTPUSER": true, "FTPPASS": true, "FTPPORT": true, "FTPDOWN": true,
}

// SecretKeys are annexure 7 keywords whose values are credentials; they
// are masked in command results.
var SecretKeys = map[string]bool{"PASS1": true, "PASS2": true, "PASSWORD": true, "FTPPASS": true}

// KnownKey reports whether k is a keyword of any table (with or without
// its instance digit) or of annexure 7.
func KnownKey(k string) bool {
	u := strings.ToUpper(k)
	if headerKeys[u] || ConfigKeys[u] || strings.HasPrefix(u, "ASN_") {
		return true
	}
	if _, ok := lookup(GroupRMS, u); ok {
		return true
	}
	base, _ := splitInstance(u)
	for g, t := range tables {
		if g == GroupRMS {
			continue
		}
		if _, ok := t[base]; ok {
			return true
		}
	}
	return false
}

func lookup(g Group, key string) (Param, bool) {
	p, ok := tables[g][key]
	return p, ok
}

// splitInstance splits an equipment keyword into the table keyword and the
// instance digit ("PDC1V1" → "PDC1V", 1; "SI101" → "SI10", 1).
func splitInstance(k string) (string, int) {
	if len(k) < 2 {
		return k, 0
	}
	c := k[len(k)-1]
	if c < '1' || c > '9' {
		return k, 0
	}
	return k[:len(k)-1], int(c - '0')
}

// uspcTable is the USPC periodic push of the 2022 "PM KUSUM National
// Portal" edition (its Annexure-8): per channel 1 to 5 of a universal solar
// pump controller, the keywords below with the channel digit appended. The
// edition prints them next to the pump controller keywords of ASN 11-19, so
// the trailing digit is the channel, not the controller instance (F-2704).
var uspcTable = map[string]Param{
	"UDKWH":   {Name: "todayEnergy", Unit: "kWh"},
	"UTOTKWH": {Name: "totalEnergy", Unit: "kWh"},
	"UDHR":    {Name: "todayRunHours", Unit: "h"},
	"UTOTHR":  {Name: "totalRunHours", Unit: "h"},
	"UOPI":    {Name: "outputCurrent", Unit: "A"},
	"UOPV":    {Name: "outputVoltage", Unit: "V"},
	"UOPKW":   {Name: "outputPower", Unit: "kW"},
	"URUNST":  {Name: "running"},
	"UFLTST":  {Name: "fault"},
}

// uspcKey reads a USPC channel keyword ("UDKWH3" → channel 3, todayEnergy).
func uspcKey(k string) (int, string, Param, bool) {
	base, ch := splitInstance(k)
	if ch == 0 || ch > 5 {
		return 0, "", Param{}, false
	}
	p, ok := uspcTable[base]
	if !ok {
		return 0, "", Param{}, false
	}
	return ch, p.Name, p, true
}

// RunStatus names the annexure 1 PRUNST values.
var RunStatus = map[int]string{
	0: "stop", 1: "running", 2: "sleep", 3: "low-speed-protection", 4: "dry-run-protection",
	5: "over-current-protection", 6: "minimum-power-protection",
}

// ControlMode names the annexure 1 PCNTRMODE values.
var ControlMode = map[int]string{0: "variable-frequency", 1: "cvt", 2: "mppt"}

func itoa(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	return itoa(i/10) + string(rune('0'+i%10))
}
