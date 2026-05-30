package relationships

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// splitASNName separates the ASN field from the name on a single line, accepting a
// pipe, tab, or first-space separator.
func splitASNName(line string) (asnField, name string) {
	if i := strings.IndexByte(line, '|'); i >= 0 {
		return strings.TrimSpace(line[:i]), strings.TrimSpace(line[i+1:])
	}
	if i := strings.IndexByte(line, '\t'); i >= 0 {
		return strings.TrimSpace(line[:i]), strings.TrimSpace(line[i+1:])
	}
	if i := strings.IndexByte(line, ' '); i >= 0 {
		return strings.TrimSpace(line[:i]), strings.TrimSpace(line[i+1:])
	}
	return strings.TrimSpace(line), ""
}

// ParseNames parses a simple ASN-name table into the builder. Each non-empty,
// non-comment line is "<asn><sep><name>" where sep is a pipe, tab, or space, and
// the ASN may carry a leading "AS".
func ParseNames(b *Builder, r io.Reader) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), scannerMaxLine)
	line := 0
	for sc.Scan() {
		line++
		text := strings.TrimSpace(strings.TrimRight(sc.Text(), "\r"))
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		asnField, name := splitASNName(text)
		asn, err := strconv.ParseUint(strings.TrimPrefix(asnField, "AS"), 10, 32)
		if err != nil {
			return fmt.Errorf("relationships: bad ASN in names at line %d: %w", line, err)
		}
		b.SetName(uint32(asn), name)
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("relationships: names read error: %w", err)
	}
	return nil
}
