package rtr

import (
	"encoding/binary"
	"fmt"
	"io"
	"net/netip"
)

const maxPDULen = 1 << 20 // 1 MiB guard against a corrupt length field

var be = binary.BigEndian

// readHeader reads and validates the 8-byte PDU header.
func readHeader(r io.Reader) (header, error) {
	var b [headerLen]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return header{}, err
	}
	h := header{
		version: b[0],
		ptype:   PDUType(b[1]),
		session: be.Uint16(b[2:4]),
		length:  be.Uint32(b[4:8]),
	}
	if h.length < headerLen || h.length > maxPDULen {
		return h, fmt.Errorf("rtr: invalid PDU length %d", h.length)
	}
	return h, nil
}

// readPDU reads one full PDU and returns a decoded value (one of the PDU structs).
func readPDU(r io.Reader) (any, error) {
	h, err := readHeader(r)
	if err != nil {
		return nil, err
	}
	body := make([]byte, h.length-headerLen)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}
	switch h.ptype {
	case PDUResetQuery:
		return ResetQuery{}, nil
	case PDUSerialQuery:
		if len(body) < 4 {
			return nil, errShortBody(h.ptype)
		}
		return SerialQuery{Session: h.session, Serial: be.Uint32(body)}, nil
	case PDUSerialNotify:
		if len(body) < 4 {
			return nil, errShortBody(h.ptype)
		}
		return SerialNotify{Session: h.session, Serial: be.Uint32(body)}, nil
	case PDUCacheResponse:
		return CacheResponse{Session: h.session}, nil
	case PDUCacheReset:
		return CacheReset{}, nil
	case PDUIPv4Prefix:
		return decodePrefix(body, false)
	case PDUIPv6Prefix:
		return decodePrefix(body, true)
	case PDUEndOfData:
		if len(body) < 16 {
			return nil, errShortBody(h.ptype)
		}
		return EndOfData{
			Session: h.session,
			Serial:  be.Uint32(body[0:]),
			Refresh: be.Uint32(body[4:]),
			Retry:   be.Uint32(body[8:]),
			Expire:  be.Uint32(body[12:]),
		}, nil
	case PDURouterKey:
		return RouterKey{}, nil
	case PDUErrorReport:
		return decodeError(h, body), nil
	default:
		return nil, fmt.Errorf("rtr: unsupported PDU type %d", h.ptype)
	}
}

func errShortBody(t PDUType) error { return fmt.Errorf("rtr: short body for PDU type %d", t) }

// decodePrefix decodes an IPv4 (12-byte) or IPv6 (28-byte) Prefix PDU body.
func decodePrefix(body []byte, v6 bool) (PrefixPDU, error) {
	addrLen := 4
	if v6 {
		addrLen = 16
	}
	if len(body) < 4+addrLen+4 {
		return PrefixPDU{}, fmt.Errorf("rtr: short prefix PDU (%d bytes)", len(body))
	}
	flags, plen, maxlen := body[0], body[1], body[2]
	var addr netip.Addr
	if v6 {
		addr = netip.AddrFrom16([16]byte(body[4:20]))
	} else {
		addr = netip.AddrFrom4([4]byte(body[4:8]))
	}
	asn := be.Uint32(body[4+addrLen:])
	prefix := netip.PrefixFrom(addr, int(plen)).Masked()
	if !prefix.IsValid() {
		return PrefixPDU{}, fmt.Errorf("rtr: invalid prefix len %d", plen)
	}
	return PrefixPDU{
		IsV6:     v6,
		Announce: flags&flagAnnounce != 0,
		Prefix:   prefix,
		MaxLen:   maxlen,
		ASN:      asn,
	}, nil
}

func decodeError(h header, body []byte) ErrorReport {
	// body: encapsulated-PDU-len(4) | encapsulated PDU | text-len(4) | text
	er := ErrorReport{Code: h.session}
	if len(body) < 4 {
		return er
	}
	encLen := be.Uint32(body)
	off := 4 + int(encLen)
	if off+4 <= len(body) {
		textLen := be.Uint32(body[off:])
		off += 4
		if off+int(textLen) <= len(body) {
			er.Text = string(body[off : off+int(textLen)])
		}
	}
	return er
}

// ---- Encoders ----

func putHeader(ptype PDUType, session uint16, length uint32) []byte {
	b := make([]byte, headerLen)
	b[0] = ProtocolVersion
	b[1] = byte(ptype)
	be.PutUint16(b[2:4], session)
	be.PutUint32(b[4:8], length)
	return b
}

func writeResetQuery(w io.Writer) error {
	_, err := w.Write(putHeader(PDUResetQuery, 0, headerLen))
	return err
}

func writeSerialQuery(w io.Writer, session uint16, serial uint32) error {
	b := putHeader(PDUSerialQuery, session, headerLen+4)
	b = be.AppendUint32(b, serial)
	_, err := w.Write(b)
	return err
}

// EncodeCacheResponse builds a Cache Response PDU (used by tests/servers).
func EncodeCacheResponse(session uint16) []byte {
	return putHeader(PDUCacheResponse, session, headerLen)
}

// EncodeSerialNotify builds a Serial Notify PDU.
func EncodeSerialNotify(session uint16, serial uint32) []byte {
	return be.AppendUint32(putHeader(PDUSerialNotify, session, headerLen+4), serial)
}

// EncodeCacheReset builds a Cache Reset PDU.
func EncodeCacheReset() []byte { return putHeader(PDUCacheReset, 0, headerLen) }

// EncodeEndOfData builds an End Of Data PDU (v1, with timers).
func EncodeEndOfData(session uint16, serial, refresh, retry, expire uint32) []byte {
	b := putHeader(PDUEndOfData, session, headerLen+16)
	b = be.AppendUint32(b, serial)
	b = be.AppendUint32(b, refresh)
	b = be.AppendUint32(b, retry)
	b = be.AppendUint32(b, expire)
	return b
}

// EncodePrefix builds an IPv4 or IPv6 Prefix PDU.
func EncodePrefix(announce bool, prefix netip.Prefix, maxLen uint8, asn uint32) []byte {
	v6 := prefix.Addr().Is6()
	ptype := PDUIPv4Prefix
	addrLen := 4
	if v6 {
		ptype = PDUIPv6Prefix
		addrLen = 16
	}
	length := uint32(headerLen + 4 + addrLen + 4)
	b := putHeader(ptype, 0, length)
	var flags byte
	if announce {
		flags = flagAnnounce
	}
	b = append(b, flags, byte(prefix.Bits()), maxLen, 0)
	if v6 {
		a := prefix.Addr().As16()
		b = append(b, a[:]...)
	} else {
		a := prefix.Addr().As4()
		b = append(b, a[:]...)
	}
	b = be.AppendUint32(b, asn)
	return b
}
