package main

import (
	"fmt"

	"CompiHospitalGO/hospital"
)

func main() {
	// 1. Create the hospital.
	h := hospital.NewHospital("Hospital de los Costeños con Narcolepsia")
	fmt.Printf("=== %s ===\n\n", h.Name())

	// 2. Hire 2 doctors and 1 orderly -> staff holds two different Attender types.
	karen := hospital.NewDoctor("Karen Ospina", 41, "Neurología")
	andres := hospital.NewDoctor("Andrés Bolaño", 37, "Neumología")
	yeison := hospital.NewOrderly("Yeison Flórez", 29)
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
	yeimy := hospital.NewPatient("Yeimy Padilla", 34, hospital.Severe)
	elkin := hospital.NewPatient("Elkin Mendoza", 50, hospital.Moderate)
	luz := hospital.NewPatient("Luz Marina Cuesta", 45, hospital.Mild)
	wilfrido := hospital.NewPatient("Wilfrido Berrio", 62, hospital.Severe)
	alexander := hospital.NewPatient("Alexander Ruiz", 28, hospital.Moderate)
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

	// The fourth one stays in the hallway: the hospital detects the state and auto-dispatches an attender.
	sleepAttack(wilfrido, "pasillo 2, segundo piso")
	for _, err := range h.AttendHallwayPatients() {
		check(err)
	}

	// --- The four required queries (Section 5), in the order of the illustrative output ---
	// Query 5.1: printed right after the fourth patient is left stuck in the hallway.
	fmt.Println("== Patients asleep in hallway ==")
	for _, p := range h.PatientsInHallway() {
		fmt.Printf("  %s %s (%s)\n", p.ID(), p.Name(), p.Location())
	}

	// Query 5.2
	fmt.Printf("== Episodes attended by Dr. %s ==\n", karen.Name())
	for _, e := range karen.MyEpisodes() {
		fmt.Printf("  %s %s\n", e.Time().Format("2006-01-02 15:04"), e.Summary())
	}

	// Query 5.3: the fourth patient fails to get a room and stays in the hallway.
	fmt.Printf("== Assigning room to %s ==\n", wilfrido.ID())
	if room, err := h.AssignRoom(wilfrido); err != nil {
		fmt.Printf("  no room available: patient stays in the hallway (%v)\n", err)
	} else {
		fmt.Printf("  %s assigned to room %d\n", wilfrido.ID(), room.ID())
	}

	// Query 5.4
	fmt.Println("== Severity report (Severe patients) ==")
	for p, count := range h.SevereReport() {
		fmt.Printf("  %s %s: %d episode(s) today\n", p.ID(), p.Name(), count)
	}

	// 6. One patient wakes up, releases the room, and the hallway patient moves in.
	fmt.Println("\n== Room release and reassignment ==")
	check(yeimy.WakeUp())
	fmt.Printf("  %s woke up and released room 101\n", yeimy.ID())
	if room, err := h.AssignRoom(wilfrido); err != nil {
		check(err)
	} else {
		fmt.Printf("  %s moved from the hallway to room %d\n", wilfrido.ID(), room.ID())
	}

	fmt.Printf("\nPatients still in the hallway: %d\n", len(h.PatientsInHallway()))
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
