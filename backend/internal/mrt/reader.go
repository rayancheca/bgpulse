package mrt

import (
	"bufio"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"strings"

	gomrt "github.com/osrg/gobgp/v4/pkg/packet/mrt"

	"github.com/rayancheca/bgpulse/backend/internal/bgp"
)

// maxMRTRecord bounds a single MRT record so a corrupt length field cannot make the
// scanner allocate without limit.
const maxMRTRecord = 16 * 1024 * 1024

// DecodeStream decodes an MRT byte stream into normalized UpdateEvents. It frames
// records with gobgp's SplitMrt; individually malformed records are skipped rather
// than aborting the whole stream.
func DecodeStream(r io.Reader) ([]bgp.UpdateEvent, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), maxMRTRecord)
	sc.Split(gomrt.SplitMrt)

	var events []bgp.UpdateEvent
	for sc.Scan() {
		data := sc.Bytes()
		if len(data) < gomrt.MRT_COMMON_HEADER_LEN {
			continue
		}
		hdr, err := gomrt.ParseHeader(data[:gomrt.MRT_COMMON_HEADER_LEN])
		if err != nil {
			continue
		}
		msg, err := gomrt.ParseBody(data[gomrt.MRT_COMMON_HEADER_LEN:], hdr)
		if err != nil {
			continue
		}
		events = append(events, recordToEvents(msg)...)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("mrt: scan: %w", err)
	}
	return events, nil
}

// OpenFile decodes an MRT dump file, transparently decompressing ".gz" and ".bz2".
func OpenFile(path string) ([]bgp.UpdateEvent, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("mrt: open %s: %w", path, err)
	}
	defer f.Close()

	var r io.Reader = f
	switch {
	case strings.HasSuffix(path, ".gz"):
		gz, err := gzip.NewReader(f)
		if err != nil {
			return nil, fmt.Errorf("mrt: gzip %s: %w", path, err)
		}
		defer gz.Close()
		r = gz
	case strings.HasSuffix(path, ".bz2"):
		r = bzip2.NewReader(f)
	}
	return DecodeStream(r)
}
