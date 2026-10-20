// Command web is the optional graphical interface (bonus, Section 7): a small
// REST API served with the standard library plus a static HTML/JS front end. It
// is a live moderation panel of the whole hospital: it shows every registered
// patient, the staff and the episode records, lets you create new entities and
// drive every method of the model (sleep a patient, let the hospital dispatch an
// attender, call diagnose, and so on).
//
// It imports the hospital package and drives it only through its exported API,
// so the model keeps no knowledge of the display and is not modified to add the
// interface: all validation stays in the model and the UI only displays the
// errors it receives. Run it from the repository root so the sprites in assets/
// are found:
//
//	go run ./web
package main

import (
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"

	"CompiHospitalGO/hospital"
)

//go:embed static
var staticFS embed.FS

// server owns the hospital and serialises every access to it. net/http handles
// each request in its own goroutine and the model has no lock of its own, so the
// web layer provides one here instead of touching the hospital package.
type server struct {
	mu sync.Mutex
	h  *hospital.Hospital
}

func main() {
	addr := flag.String("addr", ":8080", "address to listen on")
	assets := flag.String("assets", "assets", "directory that holds the sprite sheet")
	flag.Parse()

	h, err := newScenario()
	if err != nil {
		log.Fatalf("web: build scenario: %v", err)
	}
	s := &server{h: h}

	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Fatalf("web: embedded static files: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/state", s.handleState)
	mux.HandleFunc("/api/action", s.handleAction)
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir(*assets))))
	mux.Handle("/", http.FileServer(http.FS(sub)))

	log.Printf("Hospital moderation panel on http://localhost%s (sprites from %s/)", *addr, *assets)
	log.Fatal(http.ListenAndServe(*addr, mux))
}

// --- scenario -----------------------------------------------------------------

// newScenario builds the mandatory Section 6 hospital through the public API and
// seeds a few states so every sprite animation is visible as soon as the page
// loads: some patients are awake (idle), some are asleep in bed, and one is
// asleep in the hallway. It is also the starting point of the moderation panel:
// every entity can then be created, queried and mutated live from the UI.
func newScenario() (*hospital.Hospital, error) {
	h, err := hospital.NewHospital("Hospital de los Costeños con Narcolepsia")
	if err != nil {
		return nil, err
	}

	karen, err := hospital.NewDoctor("Karen Ospina", 41, "Neurología")
	if err != nil {
		return nil, err
	}
	andres, err := hospital.NewDoctor("Andrés Bolaño", 37, "Neumología")
	if err != nil {
		return nil, err
	}
	yeison, err := hospital.NewOrderly("Yeison Flórez", 29)
	if err != nil {
		return nil, err
	}
	if err := errors.Join(h.HireDoctor(karen), h.HireDoctor(andres), h.HireOrderly(yeison)); err != nil {
		return nil, err
	}

	for _, number := range []int{101, 102, 103} {
		room, err := hospital.NewRoom(number, 1)
		if err != nil {
			return nil, err
		}
		if err := h.RegisterRoom(room); err != nil {
			return nil, err
		}
	}

	yeimy, err := hospital.NewPatient("Yeimy Padilla", 34, hospital.Severe)
	if err != nil {
		return nil, err
	}
	elkin, err := hospital.NewPatient("Elkin Mendoza", 50, hospital.Moderate)
	if err != nil {
		return nil, err
	}
	luz, err := hospital.NewPatient("Luz Marina Cuesta", 45, hospital.Mild)
	if err != nil {
		return nil, err
	}
	wilfrido, err := hospital.NewPatient("Wilfrido Berrio", 62, hospital.Severe)
	if err != nil {
		return nil, err
	}
	alexander, err := hospital.NewPatient("Alexander Ruiz", 28, hospital.Moderate)
	if err != nil {
		return nil, err
	}
	for _, p := range []*hospital.Patient{yeimy, elkin, luz, wilfrido, alexander} {
		if err := h.AdmitPatient(p); err != nil {
			return nil, err
		}
	}

	// Two patients asleep in bed, one asleep in the hallway; Luz and Alexander
	// stay awake so the "sleep attack" and the diagnose flow can be tried live.
	if err := yeimy.SufferSleepAttack("cafetería"); err != nil {
		return nil, err
	}
	if _, err := h.AssignRoom(yeimy); err != nil {
		return nil, err
	}
	if _, err := h.Dispatch(yeimy, "cafetería"); err != nil {
		return nil, err
	}
	if err := karen.DiagnosePatient(yeimy); err != nil {
		return nil, err
	}
	if err := elkin.SufferSleepAttack("sala de espera"); err != nil {
		return nil, err
	}
	if _, err := h.AssignRoom(elkin); err != nil {
		return nil, err
	}
	if _, err := h.Dispatch(elkin, "sala de espera"); err != nil {
		return nil, err
	}
	if err := wilfrido.SufferSleepAttack("pasillo 2, segundo piso"); err != nil {
		return nil, err
	}
	if _, err := h.Dispatch(wilfrido, "pasillo 2, segundo piso"); err != nil {
		return nil, err
	}

	return h, nil
}

// --- HTTP handlers ------------------------------------------------------------

func (s *server) handleState(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	state := s.snapshot()
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, state)
}

type actionRequest struct {
	Type      string `json:"type"`
	Patient   string `json:"patient"`
	Location  string `json:"location"`
	Name      string `json:"name"`
	Age       int    `json:"age"`
	Level     string `json:"level"`
	Specialty string `json:"specialty"`
	Number    int    `json:"number"`
	Capacity  int    `json:"capacity"`
	Doctor    string `json:"doctor"`
}

func (s *server) handleAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "action requires POST")
		return
	}
	var req actionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	switch req.Type {
	case "admit", "sleep", "wake", "assign-room", "dispatch", "attend-hallway",
		"diagnose", "attend-sleep", "hire-doctor", "hire-orderly", "register-room", "reset":
	default:
		writeError(w, http.StatusBadRequest, fmt.Sprintf("unknown action %q", req.Type))
		return
	}

	s.mu.Lock()
	err := s.apply(req)
	state := s.snapshot()
	s.mu.Unlock()

	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, state)
}

// apply runs one action against the hospital. It is called with the lock held.
// It coordinates the model, never validates for it: every business rule lives in
// the hospital package, and whatever error comes back is forwarded unchanged so
// the front end only has to display it.
func (s *server) apply(req actionRequest) error {
	switch req.Type {
	case "admit":
		return s.admit(req)
	case "sleep":
		return s.withPatient(req.Patient, func(p *hospital.Patient) error {
			location := strings.TrimSpace(req.Location)
			if location == "" {
				location = "pasillo"
			}
			return p.SufferSleepAttack(location)
		})
	case "wake":
		// Waking a patient also reopens their case: the hospital immediately
		// dispatches an attender to attend the episode that just ended.
		return s.withPatient(req.Patient, func(p *hospital.Patient) error {
			if p.State() == hospital.Awake {
				return fmt.Errorf("patient %s wake up: %w", p.ID(), hospital.ErrIsAwake)
			}
			return p.WakeUp()
		})
	case "assign-room":
		return s.withPatient(req.Patient, func(p *hospital.Patient) error {
			_, err := s.h.AssignRoom(p)
			return err
		})
	case "dispatch":
		return s.withPatient(req.Patient, func(p *hospital.Patient) error {
			location := strings.TrimSpace(req.Location)
			if location == "" {
				location = p.Location()
			}
			_, err := s.h.Dispatch(p, location)
			return err
		})
	case "attend-hallway":
		return firstErr(s.h.AttendHallwayPatients())
	case "diagnose":
		return s.diagnose(req)
	case "attend-sleep":
		return s.attendSleep(req)
	case "hire-doctor":
		return s.hireDoctor(req)
	case "hire-orderly":
		return s.hireOrderly(req)
	case "register-room":
		return s.registerRoom(req)
	case "reset":
		h, err := newScenario()
		if err != nil {
			return err
		}
		s.h = h
		return nil
	default:
		return fmt.Errorf("unknown action %q", req.Type)
	}
}

// diagnose puts a doctor in charge of a patient (Doctor.DiagnosePatient). The
// target doctor is optional: when omitted the hospital picks the next attender
// in round-robin order, but only if it is a doctor.
func (s *server) diagnose(req actionRequest) error {
	p, err := s.patient(req.Patient)
	if err != nil {
		return err
	}
	doctor, err := s.pickDoctor(req.Doctor)
	if err != nil {
		return err
	}
	return doctor.DiagnosePatient(p)
}

// attendSleep calls Doctor.AttendSleepEmergency directly, so the "only a
// sleeping patient is a sleep emergency" rule can be exercised on its own.
func (s *server) attendSleep(req actionRequest) error {
	p, err := s.patient(req.Patient)
	if err != nil {
		return err
	}
	doctor, err := s.pickDoctor(req.Doctor)
	if err != nil {
		return err
	}
	return doctor.AttendSleepEmergency(p)
}

func (s *server) hireDoctor(req actionRequest) error {
	d, err := hospital.NewDoctor(strings.TrimSpace(req.Name), req.Age, strings.TrimSpace(req.Specialty))
	if err != nil {
		return err
	}
	return s.h.HireDoctor(d)
}

func (s *server) hireOrderly(req actionRequest) error {
	o, err := hospital.NewOrderly(strings.TrimSpace(req.Name), req.Age)
	if err != nil {
		return err
	}
	return s.h.HireOrderly(o)
}

func (s *server) registerRoom(req actionRequest) error {
	room, err := hospital.NewRoom(req.Number, req.Capacity)
	if err != nil {
		return err
	}
	return s.h.RegisterRoom(room)
}

func (s *server) pickDoctor(id string) (*hospital.Doctor, error) {
	if strings.TrimSpace(id) != "" {
		for _, d := range s.h.Doctors() {
			if d.ID() == id {
				return d, nil
			}
		}
		return nil, fmt.Errorf("doctor %q not found", id)
	}
	attender := s.h.NextAttender()
	doctor, ok := attender.(*hospital.Doctor)
	if !ok {
		return nil, fmt.Errorf("no doctor available to take charge (next attender is not a doctor)")
	}
	return doctor, nil
}

func (s *server) admit(req actionRequest) error {
	level, err := parseLevel(req.Level)
	if err != nil {
		return err
	}
	p, err := hospital.NewPatient(strings.TrimSpace(req.Name), req.Age, level)
	if err != nil {
		return err
	}
	return s.h.AdmitPatient(p)
}

func (s *server) withPatient(id string, fn func(*hospital.Patient) error) error {
	p, err := s.patient(id)
	if err != nil {
		return err
	}
	return fn(p)
}

func (s *server) patient(id string) (*hospital.Patient, error) {
	for _, p := range s.h.Patients() {
		if p.ID() == id {
			return p, nil
		}
	}
	return nil, fmt.Errorf("patient %q not found", id)
}

func parseLevel(name string) (hospital.NarcolepsyLevel, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "mild":
		return hospital.Mild, nil
	case "moderate":
		return hospital.Moderate, nil
	case "severe":
		return hospital.Severe, nil
	default:
		return hospital.Mild, fmt.Errorf("unknown narcolepsy level %q", name)
	}
}

func firstErr(errs []error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// --- JSON view of the model ---------------------------------------------------

type stateDTO struct {
	Name         string       `json:"name"`
	Staff        []staffDTO   `json:"staff"`
	Doctors      []doctorDTO  `json:"doctors"`
	Rooms        []roomDTO    `json:"rooms"`
	Patients     []patientDTO `json:"patients"`
	Hallway      []string     `json:"hallway"`
	History      []episodeDTO `json:"history"`
	SevereReport []severeDTO  `json:"severeReport"`
}

type staffDTO struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Role      string       `json:"role"`
	Age       int          `json:"age"`
	Specialty string       `json:"specialty,omitempty"`
	Episodes  []episodeDTO `json:"episodes"`
}

type doctorDTO struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Specialty string       `json:"specialty"`
	UnderCare []string     `json:"underCare"`
	Episodes  []episodeDTO `json:"episodes"`
}

type roomDTO struct {
	Number    int      `json:"number"`
	Capacity  int      `json:"capacity"`
	State     string   `json:"state"`
	Occupants []string `json:"occupants"`
}

type patientDTO struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Age      int     `json:"age"`
	Level    string  `json:"level"`
	State    string  `json:"state"`
	StateKey string  `json:"stateKey"`
	Location string  `json:"location"`
	Room     *int    `json:"room"`
	Doctor   *string `json:"doctor"`
	Episodes int     `json:"episodes"`
}

type episodeDTO struct {
	ID       string `json:"id"`
	Time     string `json:"time"`
	Summary  string `json:"summary"`
	Patient  string `json:"patient"`
	Attender string `json:"attender"`
	Location string `json:"location"`
	Room     *int   `json:"room"`
}

type severeDTO struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// snapshot reads the whole model. It is called with the lock held, so the data it
// returns is a consistent view of a single moment.
func (s *server) snapshot() stateDTO {
	h := s.h
	st := stateDTO{
		Name:     h.Name(),
		Hallway:  []string{},
		Staff:    []staffDTO{},
		Doctors:  []doctorDTO{},
		Rooms:    []roomDTO{},
		Patients: []patientDTO{},
		History:  []episodeDTO{},
	}

	for _, a := range h.Staff() {
		dto := staffDTO{ID: a.ID(), Name: a.Name(), Episodes: []episodeDTO{}}
		switch v := a.(type) {
		case *hospital.Doctor:
			dto.Role = "Doctor"
			dto.Age = v.Age()
			dto.Specialty = v.Specialty()
			for _, e := range v.MyEpisodes() {
				dto.Episodes = append(dto.Episodes, episodeToDTO(e))
			}
		case *hospital.Orderly:
			dto.Role = "Orderly"
			dto.Age = v.Age()
			for _, e := range v.EpisodesAttended() {
				dto.Episodes = append(dto.Episodes, episodeToDTO(e))
			}
		}
		st.Staff = append(st.Staff, dto)
	}

	for _, d := range h.Doctors() {
		dd := doctorDTO{ID: d.ID(), Name: d.Name(), Specialty: d.Specialty(), UnderCare: []string{}, Episodes: []episodeDTO{}}
		for _, p := range d.PatientsUnderCare() {
			dd.UnderCare = append(dd.UnderCare, p.ID())
		}
		for _, e := range d.MyEpisodes() {
			dd.Episodes = append(dd.Episodes, episodeToDTO(e))
		}
		st.Doctors = append(st.Doctors, dd)
	}

	for _, r := range h.Rooms() {
		room := roomDTO{Number: r.ID(), Capacity: r.Capacity(), State: r.State().String(), Occupants: []string{}}
		for _, p := range r.Occupants() {
			room.Occupants = append(room.Occupants, p.ID())
		}
		st.Rooms = append(st.Rooms, room)
	}

	for _, p := range h.Patients() {
		dto := patientDTO{
			ID:       p.ID(),
			Name:     p.Name(),
			Age:      p.Age(),
			Level:    p.Level().String(),
			State:    p.State().String(),
			StateKey: stateKey(p.State()),
			Location: p.Location(),
		}
		if r := p.Room(); r != nil {
			number := r.ID()
			dto.Room = &number
		}
		if d := p.Doctor(); d != nil {
			id := d.ID()
			dto.Doctor = &id
		}
		for _, e := range h.History() {
			if e.Patient() == p {
				dto.Episodes++
			}
		}
		st.Patients = append(st.Patients, dto)
	}

	for _, p := range h.PatientsInHallway() {
		st.Hallway = append(st.Hallway, p.ID())
	}

	for _, e := range h.History() {
		st.History = append(st.History, episodeToDTO(e))
	}

	report := h.SevereReport()
	severe := make([]severeDTO, 0, len(report))
	for p, count := range report {
		severe = append(severe, severeDTO{ID: p.ID(), Name: p.Name(), Count: count})
	}
	sort.Slice(severe, func(i, j int) bool {
		if severe[i].Count != severe[j].Count {
			return severe[i].Count > severe[j].Count
		}
		return severe[i].ID < severe[j].ID
	})
	st.SevereReport = severe

	return st
}

func episodeToDTO(e hospital.EpisodeRecord) episodeDTO {
	dto := episodeDTO{
		ID:       e.ID(),
		Time:     e.Time().Format("2006-01-02 15:04"),
		Summary:  e.Summary(),
		Patient:  e.Patient().ID(),
		Attender: e.Attender().ID(),
		Location: e.Location(),
	}
	if r := e.Room(); r != nil {
		number := r.ID()
		dto.Room = &number
	}
	return dto
}

// stateKey gives the front end a stable, space-free key for each patient state.
func stateKey(s hospital.PatientState) string {
	switch s {
	case hospital.Awake:
		return "Awake"
	case hospital.AsleepInHallway:
		return "AsleepInHallway"
	case hospital.AsleepInBed:
		return "AsleepInBed"
	default:
		return "Unknown"
	}
}

// --- helpers ------------------------------------------------------------------

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, message string) {
	writeJSON(w, code, map[string]string{"error": message})
}
