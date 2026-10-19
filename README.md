# Hospital de los Costeños con Narcolepsia

Hospital management system for the *Hospital*, written in Go. It showcases idiomatic Go object orientation: struct composition (`Person` embedding), polymorphism through interfaces (`Attender`), package-level encapsulation with unexported fields, typed constants (`iota`), and strict error handling.

> [!NOTE]
> Project developed by Julián Marín Ramírez for the **Lenguajes y Compiladores** course

## Requirements

- Go 1.21 or later.
- Standard library only (no third-party modules).

## How to run

From the repository root:

```bash
go run .
```

This runs the mandatory demonstration scenario (Section 6) and prints the result of the four required queries.

Expected output (excerpt):

```text
=== Hospital de los Costeños con Narcolepsia ===
Staff: 3 | Rooms: 3 | Patients: 5
  P-001 Yeimy Padilla fell asleep in cafetería
  ...
  P-004 Wilfrido Berrio fell asleep in pasillo 2, segundo piso

== Assigning room to P-004 ==
  no room available: patient stays in the hallway (assign room P-004: no free rooms at this time)

== Room release and reassignment ==
  P-001 woke up and released room 101
  P-004 moved from the hallway to room 101

== Patients asleep in hallway ==
  nobody asleep in the hallway
== Episodes attended by Dr. Karen Ospina ==
  ...
== Severity report (Severe patients) ==
  ...
```

Query 5.3 is answered in the middle of the scenario, while P-004 is still asleep in the hallway, so the "no free room" case is exercised on a genuinely sleeping patient (a bed cannot be assigned to an awake one). The remaining queries print once the scenario has finished, so Query 5.1 reports that the hallway was emptied when the woken patient freed a bed.

## How to run the tests

```bash
# every package
go test ./...

# verbose, showing each test by name
go test -v ./hospital/

# a single test
go test -v -run TestAssignRoomNoFreeRoom ./hospital/

# data-race detection
go test -race ./...
```

Expected summary:

```text
?    CompiHospitalGO           [no test files]
ok   CompiHospitalGO/hospital
```

## Tests

The suite lives in `hospital/hospital_test.go` (white-box, `package hospital`) and verifies the model's behaviour:

| Test | What it checks |
|------|----------------|
| `TestAssignRoomWithFreeRoom` | A free room is returned, linked to the patient, and the patient moves to `AsleepInBed`. |
| `TestAssignRoomNoFreeRoom` | All rooms full: wraps `ErrNoFreeRoom` and the patient stays `AsleepInHallway`. |
| `TestAssignRoomRejectsAwakePatient` | An awake patient is refused with `ErrIsAwake`. |
| `TestPatientsInHallway` | Returns only the patients `AsleepInHallway`, never those in bed. |
| `TestSevereReportCountsEpisodes` | Only `Severe` patients, each with their episode count. |
| `TestDoctorDiagnosesUntilCapacity` | Refuses the same patient (`ErrSameDoctor`) and a full doctor (`ErrDocUnavailable`). |
| `TestRoomAcceptsMultipleUntilCapacity` | Stays `Available` until full, then refuses duplicates (`ErrAlreadyInRoom`) and overflow (`ErrRoomOccupied`). |
| `TestRoomKeepsPatientLink` | `Occupy`/`Release` keep the patient's room back-reference in sync. |
| `TestDispatchRotatesThroughStaff` | Round-robin rotates the `[]Attender` (doctor, then orderly) and records each episode on the attender and in the history. |
| `TestDispatchAttendsWithDoctorAtCapacity` | A doctor at `maxPatients` is still dispatched to an emergency. |
| `TestAttendHallwayPatients` | Dispatches an attender to every patient in a hallway. |

## Design decisions

- **Composition over inheritance.** Identity data (`id`, `name`, `age`) lives in `Person`, which is embedded in `Patient` and `Doctor`, so no fields are duplicated. The embedded methods are promoted automatically.
- **Encapsulation through packages.** Every struct keeps its fields unexported and exposes them through methods; the domain rules cannot be bypassed from outside the `hospital` package.
- **Polymorphism with a real interface.** `Attender` is implemented by `Doctor` and `Orderly`, and the hospital stores its staff in a `[]Attender`. `Dispatch` calls `Attend` without any type assertion, so the concrete type is decided at runtime.
- **Room state derived from capacity.** A room is `Available` as long as it has **at least one free slot**, and it is marked `Occupied` **only when it reaches full capacity**. `Occupy` refuses to overflow capacity and refuses the same patient twice; `Release` frees a slot and refreshes the state, so a room does not need to be completely empty to accept a new patient again. `Occupy` and `Release` also own the patient's back-reference to the room, so the room and the patient can never disagree about where the patient sleeps.
- **A bed only for a sleeping patient.** `AssignRoom` refuses an awake patient with `ErrIsAwake` and a patient who already has a room with `ErrHasRoom`, so a bed can only be given to someone asleep in the hallway.
- **Validated construction.** Every constructor returns `(value, error)` and checks its own required fields before building the object: `NewHospital`, `NewPatient`, `NewDoctor` and `NewOrderly` reject a blank name (and blank specialty), a non-positive age and an out-of-range narcolepsy level, while `NewRoom` rejects a negative number or a non-positive capacity. All of them wrap the shared `ErrInvalidInput` sentinel with the specific field that failed, so an invalid entity can never be built.
- **Typed state constants.** `PatientState`, `NarcolepsyLevel` and `RoomState` are named types with `iota` constants and a `String()` method, avoiding loose strings and giving readable output.
- **Errors as values, never `panic`.** Every foreseeable failure returns a wrapped error using `fmt.Errorf("...: %w", ...)` and a sentinel error (e.g. `ErrNoFreeRoom`, `ErrRoomOccupied`), so callers can inspect them with `errors.Is`.
- **Round-robin dispatch.** A cursor (`nextAttender`) walks the `staff []Attender`, so emergencies rotate fairly across everyone hired; over several dispatches both concrete types are actually used.
- **Availability is a Doctor-only, under-care concern.** `IsAvailable` is **not** part of the `Attender` interface: it reports whether a doctor can take on another patient under care, and only `DiagnosePatient` consults it (rejecting once `maxPatients` is reached with `ErrDocUnavailable`). Dispatch never looks at it, so an emergency is always attended — `Dispatch`/`AttendHallwayPatients` rotate the next attender and `AttendSleepEmergency`/`Attend` always run, even for a doctor at capacity.
- **State-driven auto-dispatch.** `AttendHallwayPatients` scans the `AsleepInHallway` patients and dispatches an attender to each, so the hospital decides when to act based on the patient's state.
- **Queries on the final state.** The demo answers Query 5.3 in the middle of the scenario, while the fourth patient is still asleep in the hallway, so the "no free room" path runs on a genuinely sleeping patient. The remaining queries print only after the scenario ends, so Query 5.1 shows the hallway emptied once the woken patient freed a bed and the hallway patient moved in.
- **The hospital owns its history.** `Dispatch` records the episode in the hospital history itself (the attender keeps its own copy), and `RegisterEpisode` de-duplicates by episode id. An `EpisodeRecord` is a **value**, not a pointer, so each stored record is an immutable snapshot of what happened at that instant (id, time, location and room captured when it was created) instead of a handle to a shared, still-mutable object.
- **Data, not printing, in the model.** The four queries return values (`[]*Patient`, `[]EpisodeRecord`, `*Room`, `map[*Patient]int`); all printing happens in `main`.
- **Automatic identifiers.** `automaticID.go` generates unique ids per entity type with atomic counters, safe for concurrent use.
