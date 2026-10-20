// Command GUI is a small arcade game, "Coste on the ward", built on the same
// idea as the web bonus: the Go logic and a lightweight REST API on one side, an
// HTML5 canvas front end on the other. It reuses the hospital package untouched
// and only drives it through its exported API.
//
// Run it from the repository root so the sprite strips in assets/ are found:
//
//	go run ./GUI
package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"math/rand"
	"net/http"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"CompiHospitalGO/hospital"
)

//go:embed static
var staticFS embed.FS

// game owns the ward. net/http serves each request in its own goroutine and a
// background goroutine makes patients fall asleep again, so every access to the
// hospital is guarded by this mutex (the model itself is left untouched).
type game struct {
	mu     sync.Mutex
	h      *hospital.Hospital
	random *rand.Rand
}

func main() {
	addr := flag.String("addr", ":8081", "address to listen on")
	assets := flag.String("assets", "assets", "directory that holds the sprite strips")
	resleep := flag.Duration("resleep", 5*time.Second, "how often an awake patient falls asleep again (0 disables)")
	noWindow := flag.Bool("no-window", false, "do not open the game in its own application window")
	flag.Parse()

	g, err := newGame()
	if err != nil {
		log.Fatalf("GUI: build ward: %v", err)
	}
	go g.wardLoop(*resleep)

	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Fatalf("GUI: embedded static files: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/ward", g.handleWard)
	mux.HandleFunc("/api/wake", g.handleWake)
	mux.HandleFunc("/api/vallenato", g.handleVallenato)
	mux.HandleFunc("/api/dispatch", g.handleDispatch)
	mux.HandleFunc("/api/assign-room", g.handleAssignRoom)
	mux.HandleFunc("/api/diagnose", g.handleDiagnose)
	mux.HandleFunc("/api/attend-hallway", g.handleAttendHallway)
	mux.HandleFunc("/api/reset", g.handleReset)
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir(*assets))))
	mux.Handle("/", http.FileServer(http.FS(sub)))

	url := "http://localhost" + *addr
	log.Printf("Coste on the ward on %s (sprites from %s/)", url, *assets)
	if !*noWindow {
		go openAppWindow(url)
	}
	log.Fatal(http.ListenAndServe(*addr, mux))
}

// openAppWindow opens the game in a chromeless application window when a
// Chromium-family browser is available, and falls back to the default browser
// otherwise. It is best effort: if nothing can be launched the game is still
// reachable by opening the URL by hand.
func openAppWindow(url string) {
	// Give the listener a moment to come up before the window loads the page.
	time.Sleep(500 * time.Millisecond)
	for _, browser := range []string{"msedge", "chrome"} {
		path, err := exec.LookPath(browser)
		if err != nil {
			continue
		}
		cmd := exec.Command(path, "--app="+url, "--window-size=1024,760")
		if err := cmd.Start(); err == nil {
			return
		}
	}
	openDefaultBrowser(url)
}

// --- ward --------------------------------------------------------------------

var patientRoster = []struct {
	name  string
	age   int
	level hospital.NarcolepsyLevel
}{
	{"Yeimy Padilla", 34, hospital.Severe},
	{"Elkin Mendoza", 50, hospital.Moderate},
	{"Luz Marina Cuesta", 45, hospital.Mild},
	{"Wilfrido Berrio", 62, hospital.Severe},
	{"Alexander Ruiz", 28, hospital.Moderate},
	{"Nubia Carrillo", 39, hospital.Mild},
	{"Emiro Salcedo", 55, hospital.Severe},
	{"Rosa Villanueva", 47, hospital.Moderate},
	{"Jhonatan Pertuz", 23, hospital.Mild},
	{"Mileidis Ahumada", 31, hospital.Moderate},
	{"Eusebio Camargo", 60, hospital.Severe},
	{"Katerine Daza", 26, hospital.Mild},
}

var wardLocations = []string{
	"cafetería",
	"sala de espera",
	"pasillo 2, segundo piso",
	"fila de radiología",
	"pasillo de urgencias",
	"terraza de fisioterapia",
}

func newGame() (*game, error) {
	g := &game{random: rand.New(rand.NewSource(time.Now().UnixNano()))}
	if err := g.buildLocked(); err != nil {
		return nil, err
	}
	return g, nil
}

// buildLocked creates a fresh ward that mirrors the mandatory scenario of the
// statement: two doctors, one orderly, beds, and a roster of patients who fall
// asleep around the facilities. A couple of them are already asleep with a bed
// and an attended episode, so the queries have data from the first frame. Called
// with the lock held (or before the goroutines start).
func (g *game) buildLocked() error {
	h, err := hospital.NewHospital("Hospital de los Costeños con Narcolepsia")
	if err != nil {
		return err
	}

	karen, err := hospital.NewDoctor("Karen Ospina", 41, "Neurología")
	if err != nil {
		return err
	}
	andres, err := hospital.NewDoctor("Andrés Bolaño", 37, "Neumología")
	if err != nil {
		return err
	}
	yeison, err := hospital.NewOrderly("Yeison Flórez", 29)
	if err != nil {
		return err
	}
	if err := h.HireDoctor(karen); err != nil {
		return err
	}
	if err := h.HireDoctor(andres); err != nil {
		return err
	}
	if err := h.HireOrderly(yeison); err != nil {
		return err
	}

	// Beds: half a dozen single rooms, the resource the hospital fights for.
	for _, number := range []int{101, 102, 103, 104, 105, 106} {
		room, err := hospital.NewRoom(number, 1)
		if err != nil {
			return err
		}
		if err := h.RegisterRoom(room); err != nil {
			return err
		}
	}

	for _, entry := range patientRoster {
		p, err := hospital.NewPatient(entry.name, entry.age, entry.level)
		if err != nil {
			return err
		}
		if err := h.AdmitPatient(p); err != nil {
			return err
		}
	}

	g.h = h
	g.seedLocked()
	return nil
}

// seedLocked puts a few patients to sleep so the ward starts alive: the first two
// get a bed and a dispatched episode (their doctor takes charge of the case), the
// next two fall asleep in a hallway waiting for care. The rest stay awake.
func (g *game) seedLocked() {
	patients := g.h.Patients()
	asleepInBed := 2
	hallway := 2
	for i, p := range patients {
		switch {
		case i < asleepInBed:
			location := wardLocations[i%len(wardLocations)]
			if p.SufferSleepAttack(location) != nil {
				continue
			}
			if _, err := g.h.AssignRoom(p); err != nil {
				continue
			}
			if _, err := g.h.Dispatch(p, location); err != nil {
				continue
			}
			if d := g.h.Doctors(); len(d) > 0 {
				_ = d[i%len(d)].DiagnosePatient(p)
			}
		case i < asleepInBed+hallway:
			_ = p.SufferSleepAttack(wardLocations[i%len(wardLocations)])
		}
	}
}

// wardLoop keeps the ward alive. On every tick it does two things the statement
// describes: the narcolepsy makes one awake patient doze off again, and the
// hospital then takes care of whoever is left asleep in a hallway — it assigns a
// free bed when one exists and dispatches the next attender in round-robin order,
// so the player only has to keep waking people up.
func (g *game) wardLoop(every time.Duration) {
	if every <= 0 {
		return
	}
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for range ticker.C {
		g.mu.Lock()
		g.resleepLocked()
		g.careHallwayLocked()
		g.mu.Unlock()
	}
}

// resleepLocked makes one random awake patient suffer a new sleep attack.
func (g *game) resleepLocked() {
	var awake []*hospital.Patient
	for _, p := range g.h.Patients() {
		if p.State() == hospital.Awake {
			awake = append(awake, p)
		}
	}
	if len(awake) == 0 {
		return
	}
	victim := awake[g.random.Intn(len(awake))]
	_ = victim.SufferSleepAttack(wardLocations[g.random.Intn(len(wardLocations))])
}

// careHallwayLocked is the automatic side of the simulation: for every patient
// stuck in a hallway the hospital first tries to give them a bed, then dispatches
// the round-robin attender to record the episode. Failures are expected (a full
// ward, an awake patient) and simply leave the patient waiting.
func (g *game) careHallwayLocked() {
	for _, p := range g.h.PatientsInHallway() {
		if _, err := g.h.AssignRoom(p); err != nil {
			// No free bed; the patient stays in the hallway but is still attended.
			_, _ = g.h.Dispatch(p, p.Location())
			continue
		}
		_, _ = g.h.Dispatch(p, p.Location())
	}
}

// --- HTTP handlers -----------------------------------------------------------

func (g *game) handleWard(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	ward := g.wardLocked()
	g.mu.Unlock()
	writeJSON(w, http.StatusOK, ward)
}

type actionRequest struct {
	Patient  string   `json:"patient"`
	Patients []string `json:"patients"`
}

func (g *game) handleWake(w http.ResponseWriter, r *http.Request) {
	g.apply(w, r, func(req actionRequest) error {
		p, err := g.patientLocked(req.Patient)
		if err != nil {
			return err
		}
		return wake(p)
	})
}

func (g *game) handleVallenato(w http.ResponseWriter, r *http.Request) {
	g.apply(w, r, func(req actionRequest) error {
		var firstErr error
		for _, id := range req.Patients {
			p, err := g.patientLocked(id)
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			if err := wake(p); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		return firstErr
	})
}

func (g *game) handleReset(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	err := g.buildLocked()
	ward := g.wardLocked()
	g.mu.Unlock()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ward)
}

// handleDispatch sends the round-robin attender to one patient, exactly as the
// statement's emergency dispatch does. The hospital picks who goes.
func (g *game) handleDispatch(w http.ResponseWriter, r *http.Request) {
	g.apply(w, r, func(req actionRequest) error {
		p, err := g.patientLocked(req.Patient)
		if err != nil {
			return err
		}
		_, err = g.h.Dispatch(p, p.Location())
		return err
	})
}

// handleAssignRoom tries to give a sleeping patient the first free bed.
func (g *game) handleAssignRoom(w http.ResponseWriter, r *http.Request) {
	g.apply(w, r, func(req actionRequest) error {
		p, err := g.patientLocked(req.Patient)
		if err != nil {
			return err
		}
		_, err = g.h.AssignRoom(p)
		return err
	})
}

// handleDiagnose puts a doctor in charge of a patient (Doctor.DiagnosePatient).
func (g *game) handleDiagnose(w http.ResponseWriter, r *http.Request) {
	g.apply(w, r, func(req actionRequest) error {
		p, err := g.patientLocked(req.Patient)
		if err != nil {
			return err
		}
		attender := g.h.NextAttender()
		doctor, ok := attender.(*hospital.Doctor)
		if !ok {
			return fmt.Errorf("no doctor available to take charge")
		}
		return doctor.DiagnosePatient(p)
	})
}

// handleAttendHallway dispatches an attender to every patient in a hallway.
func (g *game) handleAttendHallway(w http.ResponseWriter, r *http.Request) {
	g.apply(w, r, func(req actionRequest) error {
		for _, err := range g.h.AttendHallwayPatients() {
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (g *game) apply(w http.ResponseWriter, r *http.Request, fn func(actionRequest) error) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "action requires POST")
		return
	}
	var req actionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	g.mu.Lock()
	err := fn(req)
	ward := g.wardLocked()
	g.mu.Unlock()

	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ward)
}

// wake rouses a patient if needed; waking someone already awake is a no-op so
// the client can fire it optimistically.
func wake(p *hospital.Patient) error {
	if p.State() == hospital.Awake {
		return nil
	}
	return p.WakeUp()
}

func (g *game) patientLocked(id string) (*hospital.Patient, error) {
	for _, p := range g.h.Patients() {
		if p.ID() == id {
			return p, nil
		}
	}
	return nil, fmt.Errorf("patient %q not found", id)
}

// --- JSON view ---------------------------------------------------------------

type wardDTO struct {
	Name         string       `json:"name"`
	Total        int          `json:"total"`
	Awake        int          `json:"awake"`
	Patients     []patientDTO `json:"patients"`
	Staff        []staffDTO   `json:"staff"`
	Doctors      []doctorDTO  `json:"doctors"`
	Rooms        []roomDTO    `json:"rooms"`
	Hallway      []string     `json:"hallway"`
	History      []episodeDTO `json:"history"`
	SevereReport []severeDTO  `json:"severeReport"`
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

type staffDTO struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Role      string       `json:"role"`
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

func (g *game) wardLocked() wardDTO {
	h := g.h
	ward := wardDTO{
		Name:     h.Name(),
		Patients: []patientDTO{},
		Staff:    []staffDTO{},
		Doctors:  []doctorDTO{},
		Rooms:    []roomDTO{},
		Hallway:  []string{},
		History:  []episodeDTO{},
	}

	for _, p := range h.Patients() {
		if p.State() == hospital.Awake {
			ward.Awake++
		}
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
		ward.Patients = append(ward.Patients, dto)
	}
	ward.Total = len(ward.Patients)
	sort.Slice(ward.Patients, func(i, j int) bool { return ward.Patients[i].ID < ward.Patients[j].ID })

	for _, a := range h.Staff() {
		dto := staffDTO{ID: a.ID(), Name: a.Name(), Episodes: []episodeDTO{}}
		switch v := a.(type) {
		case *hospital.Doctor:
			dto.Role = "Doctor"
			dto.Specialty = v.Specialty()
			for _, e := range v.MyEpisodes() {
				dto.Episodes = append(dto.Episodes, episodeToDTO(e))
			}
		case *hospital.Orderly:
			dto.Role = "Orderly"
			for _, e := range v.EpisodesAttended() {
				dto.Episodes = append(dto.Episodes, episodeToDTO(e))
			}
		}
		ward.Staff = append(ward.Staff, dto)
	}

	for _, d := range h.Doctors() {
		dd := doctorDTO{ID: d.ID(), Name: d.Name(), Specialty: d.Specialty(), UnderCare: []string{}, Episodes: []episodeDTO{}}
		for _, p := range d.PatientsUnderCare() {
			dd.UnderCare = append(dd.UnderCare, p.ID())
		}
		for _, e := range d.MyEpisodes() {
			dd.Episodes = append(dd.Episodes, episodeToDTO(e))
		}
		ward.Doctors = append(ward.Doctors, dd)
	}

	for _, r := range h.Rooms() {
		room := roomDTO{Number: r.ID(), Capacity: r.Capacity(), State: r.State().String(), Occupants: []string{}}
		for _, p := range r.Occupants() {
			room.Occupants = append(room.Occupants, p.ID())
		}
		ward.Rooms = append(ward.Rooms, room)
	}

	for _, p := range h.PatientsInHallway() {
		ward.Hallway = append(ward.Hallway, p.ID())
	}

	for _, e := range h.History() {
		ward.History = append(ward.History, episodeToDTO(e))
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
	ward.SevereReport = severe

	return ward
}

func episodeToDTO(e hospital.EpisodeRecord) episodeDTO {
	dto := episodeDTO{
		ID:       e.ID(),
		Time:     e.Time().Format("15:04:05"),
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

// --- helpers -----------------------------------------------------------------

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, message string) {
	writeJSON(w, code, map[string]string{"error": strings.TrimSpace(message)})
}
