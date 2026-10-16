package hospital

type Patient struct {
	Person          // embedded: Patient reuses the Person methods
	level           NarcolepsyLevel
	state           PatientState
	currentLocation string
	assignedDoctor  Doctor
}

type PatientState int

const (
	Awake PatientState = iota
	AsleepInHallway
	AsleepInBed
)

type NarcolepsyLevel int

const (
	Mild NarcolepsyLevel = iota
	Moderate
	Severe
)
