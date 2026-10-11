// Copyright 2026 Abhinav Gautam
// SPDX-License-Identifier: Apache-2.0

package jjm_test

import (
	"fmt"
	"time"

	"github.com/cB-Sigma/indiagov/jjm"
)

// A flow meter message decodes into checked values; its gid names the
// village.
func ExampleDecoder() {
	dec := jjm.Decoder{Loc: time.FixedZone("IST", 19800)}
	r, err := dec.Decode([]byte(`{"gid": "MH_D01_B02_V03", "did": "fm01", "dty": "fm", "dt": "2026-10-06T10:00:00",
		"p": {"t": "1200.5", "f": "3.2", "b": "98", "e": "00"}}`))
	if err != nil {
		panic(err)
	}
	loc, _ := jjm.ParseGID(r.GID)
	fmt.Println(r.DTY, r.Values["quantity"], r.Values["flowRate"], loc.State, loc.Village)
	// Output: fm 1200.5 3.2 MH V03
}
