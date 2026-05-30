package topology

import "github.com/rayancheca/bgpulse/backend/internal/bgp"

// ewmaAlpha weights the most recent second when smoothing the event rate.
const ewmaAlpha = 0.4

// Stats holds running counters owned by the Aggregator.
type Stats struct {
	TotalEvents  int64
	Announces    int64
	Withdraws    int64
	Leaks        int64
	Hijacks      int64
	RPKIValid    int64
	RPKIInvalid  int64
	RPKINotFound int64

	eventsPerSec float64 // EWMA over wall-clock seconds
	tickEvents   int64   // events since the last rate tick
}

// record updates counters for one classified event.
func (s *Stats) record(ev bgp.ClassifiedEvent) {
	s.TotalEvents++
	s.tickEvents++
	switch ev.Event.Kind {
	case bgp.KindAnnounce:
		s.Announces++
	case bgp.KindWithdraw:
		s.Withdraws++
	}
	switch ev.VFStatus {
	case bgp.VFLeak:
		s.Leaks++
	case bgp.VFHijack:
		s.Hijacks++
	}
	if ev.Event.Kind == bgp.KindAnnounce {
		switch ev.RPKIStatus {
		case bgp.RPKIValid:
			s.RPKIValid++
		case bgp.RPKIInvalid:
			s.RPKIInvalid++
		case bgp.RPKINotFound:
			s.RPKINotFound++
		}
	}
}

// tick folds the events seen since the last tick into the EWMA event rate. It is
// called once per wall-clock second by the aggregator.
func (s *Stats) tick() {
	s.eventsPerSec = ewmaAlpha*float64(s.tickEvents) + (1-ewmaAlpha)*s.eventsPerSec
	s.tickEvents = 0
}
