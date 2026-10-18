package main

import (
	"fmt"
	"os"

	"CompiHospitalGO/hospital"
)

func main() {
	// 1. Create the hospital.
	h := must(hospital.NewHospital("Hospital de los Costeños con Narcolepsia"))
	fmt.Printf("=== %s ===\n\n", h.Name())

	// 2. Hire 2 doctors and 1 orderly -> staff holds two different Attender types.
	karen := must(hospital.NewDoctor("Karen Ospina", 41, "Neurología"))
	andres := must(hospital.NewDoctor("Andrés Bolaño", 37, "Neumología"))
	yeison := must(hospital.NewOrderly("Yeison Flórez", 29))
	check(h.HireDoctor(karen))
	check(h.HireDoctor(andres))
	check(h.HireOrderly(yeison))

	// 3. Register 3 rooms with capacity 1.
	for _, number := range []int{101, 102, 103} {
		room, err := hospital.NewRoom(number, 1)
		check(err)
		check(h.RegisterRoom(room))
	}

	// 4. Admit 5 patients (at least two of them Severe).
	yeimy := must(hospital.NewPatient("Yeimy Padilla", 34, hospital.Severe))
	elkin := must(hospital.NewPatient("Elkin Mendoza", 50, hospital.Moderate))
	luz := must(hospital.NewPatient("Luz Marina Cuesta", 45, hospital.Mild))
	wilfrido := must(hospital.NewPatient("Wilfrido Berrio", 62, hospital.Severe))
	alexander := must(hospital.NewPatient("Alexander Ruiz", 28, hospital.Moderate))
	for _, p := range []*hospital.Patient{yeimy, elkin, luz, wilfrido, alexander} {
		check(h.AdmitPatient(p))
	}

	fmt.Printf("Staff: %d | Rooms: %d | Patients: %d\n\n", len(h.Staff()), len(h.Rooms()), len(h.Patients()))

	// 5. Four patients suffer a sleep attack in different locations.
	// The first three get a room before the attender arrives, so the episode records the room.
	sleepAttack(yeimy, "cafetería")
	assignRoom(h, yeimy)
	dispatch(h, yeimy, "cafetería")

	sleepAttack(elkin, "sala de espera")
	assignRoom(h, elkin)
	dispatch(h, elkin, "sala de espera")

	sleepAttack(luz, "fila de radiología")
	assignRoom(h, luz)
	dispatch(h, luz, "fila de radiología")

	// The fourth patient finds every bed taken. Query 5.3 (Assigning room) is
	// answered right here, while he is still asleep in the hallway, so the
	// "no room available" case is exercised on a genuinely sleeping patient.
	sleepAttack(wilfrido, "pasillo 2, segundo piso")
	fmt.Printf("\n== Assigning room to %s ==\n", wilfrido.ID())
	if room, err := h.AssignRoom(wilfrido); err != nil {
		fmt.Printf("  no room available: patient stays in the hallway (%v)\n", err)
	} else {
		fmt.Printf("  %s assigned to room %d\n", wilfrido.ID(), room.ID())
	}

	// The hospital detects the hallway state and auto-dispatches an attender to him.
	for _, err := range h.AttendHallwayPatients() {
		check(err)
	}

	// One patient wakes up, releases the room, and the hallway patient moves in.
	fmt.Println("\n== Room release and reassignment ==")
	check(yeimy.WakeUp())
	fmt.Printf("  %s woke up and released room 101\n", yeimy.ID())
	if room, err := h.AssignRoom(wilfrido); err != nil {
		check(err)
	} else {
		fmt.Printf("  %s moved from the hallway to room %d\n", wilfrido.ID(), room.ID())
	}

	// --- The remaining required queries (Section 5), on the final state ---
	// Query 5.3 was already answered above; the hallway patient has since been
	// reassigned, so Query 5.1 now reports that nobody is left asleep.

	// Query 5.1
	fmt.Println("\n== Patients asleep in hallway ==")
	if hallway := h.PatientsInHallway(); len(hallway) == 0 {
		fmt.Println("  nobody asleep in the hallway")
	} else {
		for _, p := range hallway {
			fmt.Printf("  %s %s (%s)\n", p.ID(), p.Name(), p.Location())
		}
	}

	// Query 5.2
	fmt.Printf("== Episodes attended by Dr. %s ==\n", karen.Name())
	for _, e := range karen.MyEpisodes() {
		fmt.Printf("  %s %s\n", e.Time().Format("2006-01-02 15:04"), e.Summary())
	}

	// Query 5.4
	fmt.Println("== Severity report (Severe patients) ==")
	for p, count := range h.SevereReport() {
		fmt.Printf("  %s %s: %d episode(s) today\n", p.ID(), p.Name(), count)
	}
}

// must unwraps a (value, error) pair from a validated constructor. The demo uses
// constant, valid input, so an error here means the program cannot run at all.
func must[T any](value T, err error) T {
	if err != nil {
		fmt.Printf("  (fatal) %v\n", err)
		os.Exit(1)
	}
	return value
}

func check(err error) {
	if err != nil {
		fmt.Printf("  (warning) %v\n", err)
	}
}

func sleepAttack(p *hospital.Patient, location string) {
	if err := p.SufferSleepAttack(location); err != nil {
		check(err)
		return
	}
	fmt.Printf("  %s %s fell asleep in %s\n", p.ID(), p.Name(), location)
}

func assignRoom(h *hospital.Hospital, p *hospital.Patient) {
	room, err := h.AssignRoom(p)
	if err != nil {
		check(err)
		return
	}
	fmt.Printf("  %s %s got room %d\n", p.ID(), p.Name(), room.ID())
}

func dispatch(h *hospital.Hospital, p *hospital.Patient, location string) {
	// The hospital picks any available Attender and stores the episode in its own history.
	_, err := h.Dispatch(p, location)
	check(err)
}
