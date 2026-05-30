package topology

import "time"

const (
	sparkBuckets = 30              // number of points in a throughput sparkline
	bucketWidth  = 2 * time.Second // virtual-time width of one bucket
)

// ringCounters holds a bounded, time-bucketed event count per origin AS, used to
// render the RPKI sidebar throughput sparklines. Buckets advance by event time, so
// the series is deterministic in demo mode.
type ringCounters struct {
	buckets  []int
	curIdx   int
	curStart time.Time
	started  bool
}

func newRingCounters() *ringCounters {
	return &ringCounters{buckets: make([]int, sparkBuckets)}
}

// add records one event at time t, advancing buckets as virtual time passes.
func (rc *ringCounters) add(t time.Time) {
	if !rc.started {
		rc.curStart = t
		rc.started = true
	}
	adv := int(t.Sub(rc.curStart) / bucketWidth)
	if adv < 0 {
		adv = 0 // out-of-order timestamp: fold into the current bucket
	}
	if adv >= sparkBuckets {
		for i := range rc.buckets {
			rc.buckets[i] = 0
		}
		rc.curIdx = 0
		rc.curStart = t
		rc.buckets[0]++
		return
	}
	for ; adv > 0; adv-- {
		rc.curIdx = (rc.curIdx + 1) % sparkBuckets
		rc.buckets[rc.curIdx] = 0
		rc.curStart = rc.curStart.Add(bucketWidth)
	}
	rc.buckets[rc.curIdx]++
}

// series returns the buckets oldest-to-newest as a fresh slice.
func (rc *ringCounters) series() []int {
	out := make([]int, sparkBuckets)
	for i := 0; i < sparkBuckets; i++ {
		out[i] = rc.buckets[(rc.curIdx+1+i)%sparkBuckets]
	}
	return out
}
