package relationships

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/rayancheca/bgpulse/backend/internal/bgp"
)

const scannerMaxLine = 1024 * 1024 // CAIDA lines are short; 1 MiB guards against ErrTooLong.

// caidaRel maps a CAIDA serial-2 REL code (from AS1's perspective toward AS2) to a
// RelStatus. -1: AS1 is the provider of AS2, so AS2 is AS1's customer (RelCustomer).
// 0: settlement-free peers (RelPeer). 1 (defensive; not in the canonical file):
// AS1 is the customer of AS2 (RelProvider).
func caidaRel(code int) (bgp.RelStatus, error) {
	switch code {
	case -1:
		return bgp.RelCustomer, nil
	case 0:
		return bgp.RelPeer, nil
	case 1:
		return bgp.RelProvider, nil
	default:
		return bgp.RelUnknown, fmt.Errorf("unsupported REL code %d", code)
	}
}

// ParseCAIDA parses a CAIDA AS-relationship serial-2 stream ("AS1|AS2|REL[|...]")
// into the builder. Blank lines and '#' comments are skipped; CRLF endings are
// tolerated; malformed lines (including conflicting duplicate relationships) return
// an error annotated with the 1-based line number.
func ParseCAIDA(b *Builder, r io.Reader) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), scannerMaxLine)
	line := 0
	for sc.Scan() {
		line++
		text := strings.TrimSpace(strings.TrimRight(sc.Text(), "\r"))
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		fields := strings.SplitN(text, "|", 4)
		if len(fields) < 3 {
			return fmt.Errorf("relationships: malformed line %d: %q", line, text)
		}
		as1, err := strconv.ParseUint(strings.TrimSpace(fields[0]), 10, 32)
		if err != nil {
			return fmt.Errorf("relationships: bad AS1 at line %d: %w", line, err)
		}
		as2, err := strconv.ParseUint(strings.TrimSpace(fields[1]), 10, 32)
		if err != nil {
			return fmt.Errorf("relationships: bad AS2 at line %d: %w", line, err)
		}
		code, err := strconv.Atoi(strings.TrimSpace(fields[2]))
		if err != nil {
			return fmt.Errorf("relationships: bad REL at line %d: %w", line, err)
		}
		rel, err := caidaRel(code)
		if err != nil {
			return fmt.Errorf("relationships: line %d: %w", line, err)
		}
		if err := b.Add(uint32(as1), uint32(as2), rel); err != nil {
			return fmt.Errorf("relationships: line %d: %w", line, err)
		}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("relationships: read error: %w", err)
	}
	return nil
}

// LoadCAIDA parses a CAIDA AS-relationship stream and returns an immutable RelStore.
func LoadCAIDA(r io.Reader) (*RelStore, error) {
	b := NewBuilder()
	if err := ParseCAIDA(b, r); err != nil {
		return nil, err
	}
	return b.Build(), nil
}
