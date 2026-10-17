package hospital

import "fmt"

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

func (s PatientState) String() string {
	switch s {
	case Awake:
		return "Awake"
	case AsleepInHallway:
		return "Asleep in hallway"
	case AsleepInBed:
		return "Asleep in bed"
	default:
		return "Unknown"
	}
} // for readable printing
type NarcolepsyLevel int

const (
	Mild NarcolepsyLevel = iota
	Moderate
	Severe
)

func (n NarcolepsyLevel) String() string {
	switch n {
	case Mild:
		return "Mild"
	case Moderate:
		return "Moderate"
	case Severe:
		return "Severe"
	default:
		return "Unknown"
	}
}
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

func (p *Patient) SufferSleepAttack(location string) error {
	if p.state != Awake {
		return fmt.Errorf("patient %s suffer sleep attack: %w", p.ID(), ErrIsAsleep)
	}
	p.state = AsleepInHallway
	p.currentLocation = location
	return nil
}

func (p *Patient) WakeUp() error {
	if p.state == Awake {
		return fmt.Errorf("patient %s wake up: %w", p.ID(), ErrIsAwake)
	}
	if p.room != nil {
		if err := p.room.Release(p); err != nil {
			return fmt.Errorf("patient %s wake up: %w", p.ID(), err) //do not lose the room: release it before dropping the reference
		}
	}
	p.state = Awake
	p.currentLocation = "recovery room" //send him to recovery right after
	p.room = nil
	return nil
}
