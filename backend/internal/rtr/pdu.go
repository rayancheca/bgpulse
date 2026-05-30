// Package rtr implements an RFC 8210 RPKI-to-Router (RTR) protocol-version-1 client.
// It connects to an RPKI cache (e.g. Routinator), performs the Reset/Serial Query
// synchronization dance, decodes IPv4/IPv6 Prefix PDUs into VRPs, and atomically
// swaps the validated set into an rpki.Live store. It is the live alternative to
// loading VRPs from a JSON export; the validation algorithm is identical either way.
package rtr

import "net/netip"

// ProtocolVersion is the RTR protocol version this client speaks (RFC 8210).
const ProtocolVersion uint8 = 1

const headerLen = 8

// PDUType enumerates the RTR PDU types (RFC 8210 §5).
type PDUType uint8

const (
	PDUSerialNotify  PDUType = 0
	PDUSerialQuery   PDUType = 1
	PDUResetQuery    PDUType = 2
	PDUCacheResponse PDUType = 3
	PDUIPv4Prefix    PDUType = 4
	PDUIPv6Prefix    PDUType = 6
	PDUEndOfData     PDUType = 7
	PDUCacheReset    PDUType = 8
	PDURouterKey     PDUType = 9 // BGPsec; parsed and skipped
	PDUErrorReport   PDUType = 10
)

// flagAnnounce is bit 0 of a Prefix PDU's flags: 1 = announce, 0 = withdraw.
const flagAnnounce uint8 = 1

// RTR error codes (RFC 8210 §12) handled explicitly.
const (
	ErrCorruptData                uint16 = 0
	ErrInternalError              uint16 = 1
	ErrNoDataAvailable            uint16 = 2
	ErrInvalidRequest             uint16 = 3
	ErrUnsupportedProtocolVersion uint16 = 4
	ErrUnsupportedPDUType         uint16 = 5
	ErrWithdrawalOfUnknownRecord  uint16 = 6
	ErrDuplicateAnnouncement      uint16 = 7
)

// header is the 8-byte RTR PDU header. The 16-bit field carries a session id, the
// error code, or zero depending on the PDU type.
type header struct {
	version uint8
	ptype   PDUType
	session uint16
	length  uint32
}

// Decoded PDU value types returned by readPDU.

// ResetQuery asks the cache for the full current table.
type ResetQuery struct{}

// SerialQuery asks the cache for the delta since Serial.
type SerialQuery struct {
	Session uint16
	Serial  uint32
}

// SerialNotify tells the router the cache has new data.
type SerialNotify struct {
	Session uint16
	Serial  uint32
}

// CacheResponse begins a response (full table or delta).
type CacheResponse struct {
	Session uint16
}

// PrefixPDU is an IPv4 or IPv6 prefix record (an announced or withdrawn VRP).
type PrefixPDU struct {
	IsV6     bool
	Announce bool
	Prefix   netip.Prefix
	MaxLen   uint8
	ASN      uint32
}

// EndOfData terminates a response and carries the new serial and the v1 timers.
type EndOfData struct {
	Session uint16
	Serial  uint32
	Refresh uint32
	Retry   uint32
	Expire  uint32
}

// CacheReset tells the router to discard its state and issue a Reset Query.
type CacheReset struct{}

// RouterKey is a BGPsec PDU we parse and ignore.
type RouterKey struct{}

// ErrorReport carries a fatal or transient error from the cache.
type ErrorReport struct {
	Code uint16
	Text string
}
