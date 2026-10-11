// Copyright 2026 Abhinav Gautam
// SPDX-License-Identifier: Apache-2.0

package pmkusum_test

import (
	"fmt"
	"time"

	"github.com/cB-Sigma/indiagov/pmkusum"
)

// A heartbeat-style message with a pump controller keyword decodes into
// its group, time and values.
func ExampleDecode() {
	msg := []byte(`{"VD":1,"TIMESTAMP":"2026-10-06 10:15:00","IMEI":"860000000000001","PRUNST1":"2","POPKW1":"4.50"}`)
	m, err := pmkusum.Decode(msg, time.FixedZone("IST", 19800))
	if err != nil {
		panic(err)
	}
	fmt.Println(m.Group, m.IMEI, m.Time.UTC().Format(time.RFC3339))
	fmt.Println(pmkusum.KnownKey("PRUNST1"), pmkusum.KnownKey("NOTAKEY"))
	// Output:
	// pump 860000000000001 2026-10-06T04:45:00Z
	// true false
}
