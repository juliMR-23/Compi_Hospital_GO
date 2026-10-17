package hospital

///Utility class to generate automatic IDs without duplicate code
import (
	"fmt"
	"sync/atomic"
)

type EntityType int

const (
	TypePatient EntityType = iota
	TypeDoctor
	TypeOrderly
	TypeEpisode
)

var ( //Incremental variables -> automatic ID assignation
	patientSeq int64
	doctorSeq  int64
	orderlySeq int64
	episodeSeq int64
	unknownSeq int64 //work as counters
)

/// atomic.AddInt64(&seq, 1): thread-safe increment
/// avoid 2 goroutines generating one ID at the same time -> prevent data races

func generateID(e EntityType) string {
	var id string
	switch e {
	case TypePatient:
		idNum := atomic.AddInt64(&patientSeq, 1)
		id = fmt.Sprintf("P-%03d", idNum)
	case TypeDoctor:
		idNum := atomic.AddInt64(&doctorSeq, 1)
		id = fmt.Sprintf("DOC-%03d", idNum)
	case TypeOrderly:
		idNum := atomic.AddInt64(&orderlySeq, 1)
		id = fmt.Sprintf("ORD-%03d", idNum)
	case TypeEpisode:
		idNum := atomic.AddInt64(&episodeSeq, 1)
		id = fmt.Sprintf("EP-%03d", idNum)
	default:
		idNum := atomic.AddInt64(&unknownSeq, 1)
		id = fmt.Sprintf("UNKNOWN-%03d", idNum)
	}
	return id //unique ID for every new entity
}
