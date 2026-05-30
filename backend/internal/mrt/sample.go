package mrt

import (
	"bytes"
	"fmt"
	"net/netip"
	"time"

	gobgp "github.com/osrg/gobgp/v4/pkg/packet/bgp"
	gomrt "github.com/osrg/gobgp/v4/pkg/packet/mrt"
)

// sampleBase anchors the fixture timestamps.
var sampleBase = time.Date(2024, 3, 1, 12, 0, 0, 0, time.UTC)

func ipNLRI(cidr string) (gobgp.PathNLRI, error) {
	p, err := netip.ParsePrefix(cidr)
	if err != nil {
		return gobgp.PathNLRI{}, err
	}
	pfx, err := gobgp.NewIPAddrPrefix(p)
	if err != nil {
		return gobgp.PathNLRI{}, err
	}
	return gobgp.PathNLRI{NLRI: pfx}, nil
}

func seqParam(asns ...uint32) gobgp.AsPathParamInterface {
	return gobgp.NewAs4PathParam(gobgp.BGP_ASPATH_ATTR_TYPE_SEQ, asns)
}

func setParam(asns ...uint32) gobgp.AsPathParamInterface {
	return gobgp.NewAs4PathParam(gobgp.BGP_ASPATH_ATTR_TYPE_SET, asns)
}

func nextHopAttr(ip string) (gobgp.PathAttributeInterface, error) {
	return gobgp.NewPathAttributeNextHop(netip.MustParseAddr(ip))
}

// BuildSampleMRT produces a small but real BGP4MP (MESSAGE_AS4) MRT byte stream
// containing: a normal IPv4 announce, an AS_SET-aggregated announce, an IPv6
// MP_REACH announce, and an IPv4 withdrawal. It is both the bundled replay fixture
// and the golden input for the parser tests, so the parser is exercised against
// genuine MRT/BGP wire bytes (produced by gobgp's encoder).
func BuildSampleMRT() ([]byte, error) {
	var buf bytes.Buffer
	ts := sampleBase
	add := func(peerAS uint32, peerIP string, msg *gobgp.BGPMessage) error {
		body, err := gomrt.NewBGP4MPMessage(peerAS, 65000, 0,
			netip.MustParseAddr(peerIP), netip.MustParseAddr("192.0.2.254"), true, msg)
		if err != nil {
			return fmt.Errorf("bgp4mp record: %w", err)
		}
		m, err := gomrt.NewMRTMessage(ts, gomrt.BGP4MP, gomrt.MESSAGE_AS4, body)
		if err != nil {
			return fmt.Errorf("mrt message: %w", err)
		}
		b, err := m.Serialize()
		if err != nil {
			return fmt.Errorf("serialize: %w", err)
		}
		buf.Write(b)
		ts = ts.Add(time.Second)
		return nil
	}

	// 1) Normal IPv4 announce 10.0.0.0/24 via AS_PATH 65001 3356 174 (origin 174).
	n1, err := ipNLRI("10.0.0.0/24")
	if err != nil {
		return nil, err
	}
	nh1, err := nextHopAttr("192.0.2.1")
	if err != nil {
		return nil, err
	}
	attrs1 := []gobgp.PathAttributeInterface{
		gobgp.NewPathAttributeAsPath([]gobgp.AsPathParamInterface{seqParam(65001, 3356, 174)}),
		nh1,
		gobgp.NewPathAttributeCommunities([]uint32{0x0A2C0064}), // 2604:100
	}
	if err := add(65001, "192.0.2.1", gobgp.NewBGPUpdateMessage(nil, attrs1, []gobgp.PathNLRI{n1})); err != nil {
		return nil, err
	}

	// 2) AS_SET-aggregated announce 20.0.0.0/16; trailing AS_SET makes origin unverifiable.
	n2, err := ipNLRI("20.0.0.0/16")
	if err != nil {
		return nil, err
	}
	nh2, err := nextHopAttr("192.0.2.2")
	if err != nil {
		return nil, err
	}
	attrs2 := []gobgp.PathAttributeInterface{
		gobgp.NewPathAttributeAsPath([]gobgp.AsPathParamInterface{
			seqParam(65002, 3356), setParam(64500, 64501),
		}),
		nh2,
	}
	if err := add(65002, "192.0.2.2", gobgp.NewBGPUpdateMessage(nil, attrs2, []gobgp.PathNLRI{n2})); err != nil {
		return nil, err
	}

	// 3) IPv6 announce 2001:db8::/32 via MP_REACH_NLRI (origin 6939).
	n3, err := ipNLRI("2001:db8::/32")
	if err != nil {
		return nil, err
	}
	mp, err := gobgp.NewPathAttributeMpReachNLRI(gobgp.RF_IPv6_UC,
		[]gobgp.PathNLRI{n3}, netip.MustParseAddr("2001:db8::1"))
	if err != nil {
		return nil, err
	}
	attrs3 := []gobgp.PathAttributeInterface{
		gobgp.NewPathAttributeAsPath([]gobgp.AsPathParamInterface{seqParam(65003, 6939)}),
		mp,
	}
	if err := add(65003, "192.0.2.3", gobgp.NewBGPUpdateMessage(nil, attrs3, nil)); err != nil {
		return nil, err
	}

	// 4) IPv4 withdrawal of 10.0.0.0/24.
	w1, err := ipNLRI("10.0.0.0/24")
	if err != nil {
		return nil, err
	}
	if err := add(65001, "192.0.2.1", gobgp.NewBGPUpdateMessage([]gobgp.PathNLRI{w1}, nil, nil)); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
