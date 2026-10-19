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
	resleep := flag.Duration("resleep", 6*time.Second, "how often an awake patient falls asleep again (0 disables)")
	flag.Parse()

	g, err := newGame()
	if err != nil {
		log.Fatalf("GUI: build ward: %v", err)
	}
	go g.narcolepsyLoop(*resleep)

	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Fatalf("GUI: embedded static files: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/ward", g.handleWard)
	mux.HandleFunc("/api/wake", g.handleWake)
	mux.HandleFunc("/api/vallenato", g.handleVallenato)
	mux.HandleFunc("/api/reset", g.handleReset)
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir(*assets))))
	mux.Handle("/", http.FileServer(http.FS(sub)))

	log.Printf("Coste on the ward on http://localhost%s (assets from %s/)", *addr, *assets)
	log.Fatal(http.ListenAndServe(*addr, mux))
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

// buildLocked creates a fresh ward with every patient asleep. Called with the
// lock held (or before the goroutines start).
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

	for i, entry := range patientRoster {
		p, err := hospital.NewPatient(entry.name, entry.age, entry.level)
		if err != nil {
			return err
		}
		if err := h.AdmitPatient(p); err != nil {
			return err
		}
		if err := p.SufferSleepAttack(wardLocations[i%len(wardLocations)]); err != nil {
			return err
		}
	}

	g.h = h
	return nil
}

// narcolepsyLoop keeps the game alive: every interval one of the awake patients
// falls asleep again, which is exactly the condition the hospital treats.
func (g *game) narcolepsyLoop(every time.Duration) {
	if every <= 0 {
		return
	}
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for range ticker.C {
		g.mu.Lock()
		var awake []*hospital.Patient
		for _, p := range g.h.Patients() {
			if p.State() == hospital.Awake {
				awake = append(awake, p)
			}
		}
		if len(awake) > 0 {
			victim := awake[g.random.Intn(len(awake))]
			_ = victim.SufferSleepAttack(wardLocations[g.random.Intn(len(wardLocations))])
		}
		g.mu.Unlock()
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
	Name     string       `json:"name"`
	Total    int          `json:"total"`
	Awake    int          `json:"awake"`
	Patients []patientDTO `json:"patients"`
	Staff    []staffDTO   `json:"staff"`
}

type patientDTO struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Age      int    `json:"age"`
	Level    string `json:"level"`
	State    string `json:"state"`
	StateKey string `json:"stateKey"`
	Location string `json:"location"`
}

type staffDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

func (g *game) wardLocked() wardDTO {
	ward := wardDTO{Name: g.h.Name(), Patients: []patientDTO{}, Staff: []staffDTO{}}
	for _, p := range g.h.Patients() {
		if p.State() == hospital.Awake {
			ward.Awake++
		}
		ward.Patients = append(ward.Patients, patientDTO{
			ID:       p.ID(),
			Name:     p.Name(),
			Age:      p.Age(),
			Level:    p.Level().String(),
			State:    p.State().String(),
			StateKey: stateKey(p.State()),
			Location: p.Location(),
		})
	}
	ward.Total = len(ward.Patients)
	sort.Slice(ward.Patients, func(i, j int) bool { return ward.Patients[i].ID < ward.Patients[j].ID })

	for _, a := range g.h.Staff() {
		role := "Orderly"
		if _, ok := a.(*hospital.Doctor); ok {
			role = "Doctor"
		}
		ward.Staff = append(ward.Staff, staffDTO{ID: a.ID(), Name: a.Name(), Role: role})
	}
	return ward
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
