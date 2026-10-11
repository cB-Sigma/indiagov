package jjm

import (
	"fmt"
	"math"
)

// Unit normalisation: every parameter is recorded in one unit, the one the
// report gives it (ground water depth in metres, F-481), whatever unit a
// device sends; a converted value keeps the value and unit as sent in the
// telemetry entry's raw member. The units a device sends are the report's
// unless settings say otherwise (units by parameter code, per IPE or per
// device; groundWaterUnit for g).

// RawValue is a value as sent, with its unit.
type RawValue struct {
	V float64 `json:"v"`
	U string  `json:"u"`
}

// unitTables give each unit's factor to the canonical unit of its kind.
var unitTables = map[string]map[string]float64{
	"length":        {"m": 1, "ft": 0.3048, "cm": 0.01, "mm": 0.001},
	"volume":        {"L": 1, "kL": 1000, "m3": 1000},
	"flow":          {"L/min": 1, "L/s": 60, "L/h": 1.0 / 60, "m3/h": 1000.0 / 60, "kL/h": 1000.0 / 60},
	"pressure":      {"bar": 1, "kPa": 0.01, "MPa": 10, "psi": 0.0689475729, "mH2O": 0.0980665},
	"concentration": {"mg/L": 1, "ppm": 1, "ug/L": 0.001, "ppb": 0.001},
	"conductivity":  {"uS/cm": 1, "mS/cm": 1000},
}

// CheckUnit reports whether unit is one code's value may be sent in.
func CheckUnit(code, unit string) error {
	p, ok := Params[code]
	if !ok || code == "e" {
		return fmt.Errorf("%q is not a parameter code", code)
	}
	if unit == p.Unit {
		return nil
	}
	if _, ok := unitTables[p.Kind][unit]; !ok {
		return fmt.Errorf("%s cannot be sent in %q", code, unit)
	}
	return nil
}

func (d Decoder) unitOf(code string) string {
	if u, ok := d.Units[code]; ok && u != "" {
		return u
	}
	if code == "g" {
		if d.GroundWaterUnit == "" {
			return "ft"
		}
		return d.GroundWaterUnit
	}
	return Params[code].Unit
}

func convert(p Param, from string, v float64) float64 {
	f, ok := unitTables[p.Kind][from]
	if !ok {
		return v
	}
	return math.Round(v*f*1e6) / 1e6
}
