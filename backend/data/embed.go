// Package data embeds the bundled offline fixtures so the binary is self-contained
// and runs with zero external files. Loaders accept an external path when one is
// configured and fall back to these embedded bytes otherwise.
package data

import _ "embed"

//go:embed updates.sample.mrt
var sampleMRT []byte

//go:embed as-rel.sample.txt
var sampleASRel []byte

//go:embed demo_vrps.json
var sampleVRPs []byte

// SampleMRT returns the bundled BGP4MP MRT replay fixture.
func SampleMRT() []byte { return sampleMRT }

// SampleASRel returns the bundled CAIDA AS-relationship subset.
func SampleASRel() []byte { return sampleASRel }

// SampleVRPs returns the bundled Routinator-style VRP JSON.
func SampleVRPs() []byte { return sampleVRPs }
