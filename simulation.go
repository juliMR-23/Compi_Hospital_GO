package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"CompiHospitalGO/hospital"
)

// simulationLocations are the places where a patient can suddenly fall asleep.
var simulationLocations = []string{
	"cafetería",
	"sala de espera",
	"fila de radiología",
	"pasillo 2, segundo piso",
	"jardín interior",
}

// runConcurrentSimulation is the optional concurrency bonus (Section 7): every
// patient is a goroutine that falls asleep at a random interval and then competes
// for a bed, while the Hospital serialises each access to its shared state with a
// sync.Mutex. It uses its own hospital so the graded demonstration above keeps its
// deterministic output and its history untouched. Run it with `go run -race .` to
// check that the state is race-free.
func runConcurrentSimulation() {
	fmt.Println("\n== Bonus: concurrent simulation ==")

	sim := must(hospital.NewHospital("Hospital de los Costeños (concurrent run)"))

	for _, name := range []string{"Doctor A", "Doctor B"} {
		check(sim.HireDoctor(must(hospital.NewDoctor(name, 40, "General"))))
	}
	for _, name := range []string{"Orderly A", "Orderly B"} {
		check(sim.HireOrderly(must(hospital.NewOrderly(name, 30))))
	}
	for i := 0; i < 4; i++ {
		check(sim.RegisterRoom(must(hospital.NewRoom(200+i, 1))))
	}

	const patients = 8
	const rounds = 3

	var wg sync.WaitGroup
	for i := 0; i < patients; i++ {
		p := must(hospital.NewPatient(fmt.Sprintf("Concurrent Patient %d", i+1), 30, hospital.Moderate))
		check(sim.AdmitPatient(p))

		wg.Add(1)
		go func(p *hospital.Patient) {
			defer wg.Done()
			for r := 0; r < rounds; r++ {
				time.Sleep(time.Duration(rand.Intn(5)) * time.Millisecond)
				location := simulationLocations[rand.Intn(len(simulationLocations))]
				if err := sim.SleepAttack(p, location); err != nil {
					continue // already asleep from a previous round
				}
				_, _ = sim.AssignRoom(p)         // a bed may be taken; the error just means "stays in the hallway"
				_, _ = sim.Dispatch(p, location) // an attender attends the emergency regardless of load
				_ = sim.WakeUp(p)                // free the bed so another patient can take it
			}
		}(p)
	}
	wg.Wait()

	// The summary runs on the main goroutine only, after every worker has stopped.
	placed := 0
	awake := 0
	for _, p := range sim.Patients() {
		if p.Room() != nil {
			placed++
		} else if p.State() == hospital.Awake {
			awake++
		}
	}

	fmt.Printf("  %d patients ran concurrently into %d rooms\n", patients, len(sim.Rooms()))
	fmt.Printf("  episodes recorded: %d | in a room: %d | in a hallway: %d | awake: %d\n",
		len(sim.History()), placed, len(sim.PatientsInHallway()), awake)
}
