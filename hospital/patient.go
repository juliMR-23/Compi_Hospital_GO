package hospital

type Patient struct {
	Person          // embedded: Patient reuses the Person methods
	level           NarcolepsyLevel
	state           PatientState
	currentLocation string
	assignedDoctor  *Doctor
	room            *Room //add room -> easy access when creating an Episode or transferring
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

func NewPatient(name string, age int, level NarcolepsyLevel) *Patient {
	return &Patient{
		Person:          NewPerson(generateID(TypePatient), name, age),
		level:           level,
		currentLocation: "reception", //new patient -> no room yet
		//room is nil by default
		//PatientState is Awake by default
	}
}

func (p *Patient) Level() NarcolepsyLevel { return p.level }
func (p *Patient) State() PatientState    { return p.state }
func (p *Patient) Location() string       { return p.currentLocation }
func (p *Patient) Doctor() *Doctor        { return p.assignedDoctor }
func (p *Patient) Room() *Room            { return p.room }
