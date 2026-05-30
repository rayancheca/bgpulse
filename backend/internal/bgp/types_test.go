package bgp

import "testing"

func TestUpdateKindString(t *testing.T) {
	cases := map[UpdateKind]string{
		KindAnnounce: "announce",
		KindWithdraw: "withdraw",
		UpdateKind(99): "unknown",
	}
	for k, want := range cases {
		if got := k.String(); got != want {
			t.Errorf("UpdateKind(%d).String() = %q, want %q", k, got, want)
		}
	}
}

func TestRelStatusString(t *testing.T) {
	cases := map[RelStatus]string{
		RelUnknown:  "unknown",
		RelCustomer: "customer",
		RelProvider: "provider",
		RelPeer:     "peer",
		RelSibling:  "sibling",
	}
	for r, want := range cases {
		if got := r.String(); got != want {
			t.Errorf("RelStatus(%d).String() = %q, want %q", r, got, want)
		}
	}
}

func TestRelStatusInvert(t *testing.T) {
	// customer and provider swap; the rest are self-inverse.
	if RelCustomer.Invert() != RelProvider {
		t.Errorf("RelCustomer.Invert() = %v, want RelProvider", RelCustomer.Invert())
	}
	if RelProvider.Invert() != RelCustomer {
		t.Errorf("RelProvider.Invert() = %v, want RelCustomer", RelProvider.Invert())
	}
	for _, r := range []RelStatus{RelUnknown, RelPeer, RelSibling} {
		if r.Invert() != r {
			t.Errorf("%v.Invert() = %v, want self", r, r.Invert())
		}
	}
	// Inverting twice is the identity for every value.
	for _, r := range []RelStatus{RelUnknown, RelCustomer, RelProvider, RelPeer, RelSibling} {
		if r.Invert().Invert() != r {
			t.Errorf("%v.Invert().Invert() = %v, want self", r, r.Invert().Invert())
		}
	}
}

func TestVFStatusString(t *testing.T) {
	cases := map[VFStatus]string{
		VFValid:   "valid",
		VFLeak:    "leak",
		VFHijack:  "hijack",
		VFUnknown: "unknown",
	}
	for v, want := range cases {
		if got := v.String(); got != want {
			t.Errorf("VFStatus(%d).String() = %q, want %q", v, got, want)
		}
	}
}

func TestVFStatusSeverityOrdering(t *testing.T) {
	// hijack > leak > valid > unknown — the aggregator relies on this ordering.
	if !(VFHijack.Severity() > VFLeak.Severity() &&
		VFLeak.Severity() > VFValid.Severity() &&
		VFValid.Severity() > VFUnknown.Severity()) {
		t.Errorf("severity ordering broken: hijack=%d leak=%d valid=%d unknown=%d",
			VFHijack.Severity(), VFLeak.Severity(), VFValid.Severity(), VFUnknown.Severity())
	}
}

func TestRPKIStatusString(t *testing.T) {
	cases := map[RPKIStatus]string{
		RPKINotFound: "notfound",
		RPKIValid:    "valid",
		RPKIInvalid:  "invalid",
	}
	for s, want := range cases {
		if got := s.String(); got != want {
			t.Errorf("RPKIStatus(%d).String() = %q, want %q", s, got, want)
		}
	}
}
