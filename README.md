# Hospital de los Costeños con Narcolepsia

Hospital management system for the *Hospital*, written in Go. It showcases idiomatic Go object orientation: struct composition (`Person` embedding), polymorphism through interfaces (`Attender`), package-level encapsulation with unexported fields, typed constants (`iota`), and strict error handling.

> [!NOTE]
> Project developed by Julián Marín Ramírez for the **Lenguajes y Compiladores** course

## Project structure

```
CompiHospitalGO/
├── main.go              demo scenario and all printing
├── simulation.go        optional concurrent simulation (bonus)
└── hospital/
    ├── hospital.go      Hospital: admit/hire, dispatch, AssignRoom, queries
    ├── attender.go      Attender interface (implemented by Doctor and Orderly)
    ├── person.go        embedded identity base (id, name, age)
    ├── patient.go       states, sleep attack, room back-reference
    ├── doctor.go        maxPatients, DiagnosePatient, Attend
    ├── orderly.go       transfer attender
    ├── room.go          capacity, Occupy/Release, derived state
    ├── episodeRecord.go immutable episode snapshot
    ├── errors.go        sentinel errors
    └── automaticID.go   atomic id counters
```

## Domain model

| Type | Responsibility |
|------|----------------|
| `Person` | Embedded base: `id`, `name`, `age`, shared by staff and patients. |
| `Attender` | Interface (`ID`, `Name`, `Attend`) implemented by `Doctor` and `Orderly`. |
| `Patient` | State (`Awake`/`AsleepInHallway`/`AsleepInBed`), narcolepsy level, assigned doctor and room. |
| `Doctor` | An `Attender`; takes up to `maxPatients` under care via `DiagnosePatient`. |
| `Orderly` | An `Attender` that transfers patients to rooms. |
| `Room` | Capacity, occupant list and the derived `Available`/`Occupied` state. |
| `EpisodeRecord` | Immutable snapshot of one attended episode. |
| `Hospital` | Owns the patients, staff, rooms and history; coordinates every operation. |

## Requirements

- Go 1.21 or later.
- Standard library only (no third-party modules).
- Optional: a C toolchain (e.g. MinGW-w64/TDM-GCC) with `CGO_ENABLED=1` to run the `-race` checks of the concurrent simulation.

## How to run

From the repository root:

```bash
go run .
```

This runs the mandatory demonstration scenario (Section 6) and prints the result of the four required queries, then — as an optional bonus — a concurrent simulation (see [Bonus: concurrent simulation](#bonus-concurrent-simulation)).

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

> ```text
> ?    CompiHospitalGO           [no test files]
> ok   CompiHospitalGO/hospital
> ```

## Bonus: concurrent simulation

`main` ends by running an optional concurrent simulation (`simulation.go`). Each patient is a goroutine that falls asleep at a random interval and then competes for a bed, while the `Hospital` serialises every access to its shared state with a `sync.Mutex`. It uses a separate hospital, so the graded demonstration above keeps its deterministic output and its own history untouched.

Data-race detection needs cgo and a C toolchain:

```bash
# with a C compiler (MinGW-w64 / TDM-GCC) on the PATH
go run -race .          # PowerShell: set $env:CGO_ENABLED=1 first
go test -race ./...
```

With `CGO_ENABLED=0` (the default on Windows) the `-race` build is unavailable; the program still runs and the mutex still guards the state. `TestHospitalConcurrentAccess` runs the writers and the query readers from many goroutines at once and asserts the invariants that must hold for any interleaving, so it is the deterministic companion to the simulation (best run under `-race`).

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
| `TestHospitalConcurrentAccess` | Many goroutines admit, put to sleep, assign rooms and dispatch while readers run the queries; asserts the invariants (meant for `-race`). |

## Design decisions

| Decision | Lives in | Why it matters |
|----------|----------|----------------|
| Composition over inheritance | `person.go` | identity fields are shared, never duplicated |
| Encapsulation | unexported fields | domain rules cannot be bypassed from outside |
| Polymorphism | `attender.go` + `[]Attender` | the concrete attender is chosen at runtime |
| Typed state constants | `patient.go`, `room.go` | no loose strings, readable output |
| Automatic identifiers | `automaticID.go` | unique ids, safe under concurrency |
| Room state derived from capacity | `room.go` | one source of truth for available/occupied |
| A bed only for a sleeping patient | `hospital.go` `AssignRoom` | the state is a precondition, not a guess |
| Room owns the patient↔room link | `room.go` | the two sides cannot disagree |
| Validated construction | every constructor | an invalid entity can never exist |
| Errors as values | `errors.go` | callers use `errors.Is`, never `panic` |
| Round-robin dispatch | `hospital.go` `NextAttender` | fair rotation, no starvation |
| Availability is diagnose-only | `doctor.go` | emergencies are always attended |
| State-driven auto-dispatch | `hospital.go` | the hospital reacts to the patient's state |
| History as value snapshots | `episodeRecord.go` | records stay immutable |
| Data, not printing, in the model | the four queries | the model stays UI-agnostic |
| One mutex per hospital | `hospital.go` | every exported method locks; `...Locked` cores never re-lock |

### Object-oriented foundations

- **Composition over inheritance.** Identity data (`id`, `name`, `age`) lives in `Person`, embedded in `Patient` and `Doctor`, so no fields are duplicated and the methods are promoted automatically.
- **Encapsulation through packages.** Every struct keeps its fields unexported and exposes them only through methods; the domain rules cannot be bypassed from outside the `hospital` package.
- **Polymorphism with a real interface.** `Attender` is implemented by `Doctor` and `Orderly`, and the hospital stores its staff as `[]Attender`. `Dispatch` calls `Attend` with no type assertion, so the concrete type is resolved at runtime.
- **Typed state constants.** `PatientState`, `NarcolepsyLevel` and `RoomState` are named types with `iota` constants and a `String()` method, avoiding loose strings and giving readable output.
- **Automatic identifiers.** `automaticID.go` generates unique ids per entity type with atomic counters, safe for concurrent use.

### State and invariants

- **A bed only for a sleeping patient.** `AssignRoom` refuses an awake patient (`ErrIsAwake`) and a patient who already has a room (`ErrHasRoom`), so a bed is only ever given to someone asleep in the hallway.
- **Room owns the patient↔room link.** `Room.Occupy`/`Release` update the patient's back-reference, so the room and the patient can never disagree about where the patient sleeps.
- **Validated construction.** Every constructor returns `(value, error)` and checks its own required fields (`NewRoom` rejects a negative number or a non-positive capacity; the others reject a blank name, a blank specialty, a non-positive age or an out-of-range level), all wrapping the shared `ErrInvalidInput` — so an invalid entity can never be built.

<details>
<summary><b>Room state derived from capacity</b></summary>

A room is `Available` as long as it has **at least one free slot**, and it is marked `Occupied` **only when it reaches full capacity**. `Occupy` refuses to overflow capacity and refuses the same patient twice; `Release` frees a slot and refreshes the state, so a room does not need to be completely empty to accept a new patient again.
</details>

### Error handling

- **Errors as values, never `panic`.** Every foreseeable failure returns a wrapped error using `fmt.Errorf("...: %w", ...)` and a sentinel error (e.g. `ErrNoFreeRoom`, `ErrRoomOccupied`), so callers can inspect them with `errors.Is`.

### Dispatch and availability

- **Round-robin dispatch.** A cursor (`nextAttender`) walks the `staff []Attender`, so emergencies rotate fairly across everyone hired and, over several dispatches, both concrete types are actually used.
- **State-driven auto-dispatch.** `AttendHallwayPatients` scans the `AsleepInHallway` patients and dispatches an attender to each, so the hospital decides when to act based on the patient's state.

<details>
<summary><b>Availability is a Doctor-only, under-care concern</b></summary>

`IsAvailable` is **not** part of the `Attender` interface: it reports whether a doctor can take on another patient under care, and only `DiagnosePatient` consults it (rejecting once `maxPatients` is reached with `ErrDocUnavailable`). Dispatch never looks at it, so an emergency is always attended — `Dispatch`/`AttendHallwayPatients` rotate the next attender and `AttendSleepEmergency`/`Attend` always run, even for a doctor at capacity.
</details>

### Concurrency

- **One mutex, no reentrancy.** `Hospital` holds a `sync.Mutex` and every exported method takes it before touching shared state. Internal helpers suffixed `Locked` assume the lock is already held and never lock again, so the nested calls (`AttendHallwayPatients → Dispatch → RegisterEpisode`) cannot self-deadlock on a non-reentrant mutex.
- **Mutations go through the hospital.** The simulation and the test change patient state via `Hospital.SleepAttack`/`WakeUp` rather than the entity methods directly, so a single lock covers patients, rooms, staff and history alike. Identifier generation is already lock-free (`atomic`).

### History and queries

- **The hospital owns its history.** `Dispatch` records the episode and `RegisterEpisode` de-duplicates by episode id; an `EpisodeRecord` is a **value**, so each stored record is an immutable snapshot (id, time, location and room captured when it was created) instead of a handle to a shared, still-mutable object.
- **Queries on the final state.** Query 5.3 is answered in the middle of the scenario on a genuinely sleeping patient, and the remaining queries print once it has finished.
- **Data, not printing, in the model.** The four queries return values (`[]*Patient`, `[]EpisodeRecord`, `*Room`, `map[*Patient]int`); all printing happens in `main`.
