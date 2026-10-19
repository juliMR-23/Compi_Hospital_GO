# AI Usage Documentation

## Tools Used
- **Gemini**: Initial project scaffolding, struct design guidance, interface patterns
- **JetBrains AI assistant (GoLand)**: Code completion for getter methods and boilerplate

## Key Prompts
1. "How to handle inheritance (non-native in GO) with embedding"
2. "Help me use counters for automatic ID generation for every entity"
3. "How do you implement an interface in GO? Attender interface with Doctor and Orderly types"
4. "Serve the hospital over the standard library net/http and animate a 14x14 sprite sheet with HTML/CSS"
5. "Turn the ward into a small canvas game where Coste wakes sleeping patients with vallenato, reusing the same hospital package"

## Lessons Learned - AI Generated Code Issues
- **Round-robin dispatch.** The first `Dispatch` took the first attender whose `IsAvailable()` was `true`; since the `Orderly` was always available it handled every call and the doctors never attended. I added a `nextAttender` cursor over `staff []Attender` to rotate fairly, and — as `Dispatch` only handles emergencies — moved `IsAvailable` off the `Attender` interface onto `Doctor`, where it gates `DiagnosePatient` only.
- **Room capacity not enforced.** `Occupy` just appended and set `Occupied`, so a room could overfill and one patient could take two slots. I made `NewRoom` reject `capacity <= 0`, `Occupy` refuse overflow (`ErrRoomOccupied`) and duplicates (`ErrAlreadyInRoom`), and derived the state from `len(occupants)`.
- **Bidirectional link guarded on one side.** I had `Room.Occupy`/`Release` update the patient's `room` back-reference; the AI then extended that single-owner idea across the code, removing the duplicate mutations in `AssignRoom`/`WakeUp`.
- **Room identity by pointer instead of number.** `RegisterRoom` deduplicated with `slices.Contains` (pointer identity), so two `Room` objects with the same number both registered. I switched to the room number (`slices.ContainsFunc` on `ID()`) and return `ErrRoomAlreadyRegistered`.
- **Invalid entities could be built.** Constructors trusted their inputs and `NewRoom` had a bespoke `ErrInvalidRoomCapacity`. I made every constructor return `(value, error)`, validate its fields, and wrap one shared `ErrInvalidInput`.
- **Episode history kept as a shared pointer.** `NewEpisode` returned a `*EpisodeRecord` stored in several places, so a record was a live handle to a mutable object. I made it a value (`NewEpisode` returns `EpisodeRecord`, stored as `[]EpisodeRecord`), so each record is a snapshot.
- **Display concerns leaking into the model.** For the web bonus the AI wanted to add the lock and sleep/wake helpers straight into `Hospital`. I kept the model untouched and put the `sync.Mutex` and the HTTP handlers in the `web` package instead, so the interface is added without changing the domain (the hospital package contains no display code).
- **Sprite strips: frame counts and offsets.** The AI assumed the animations all lived on one shared sheet; the real assets are one strip per animation with different frame counts (the vallenato idle alone has 23). I measured each PNG (width / frame width) to get the exact count, drove the CSS from custom properties, and gave every actor a negative `animation-delay` offset so the idles never run in sync.
- **A background goroutine racing the handlers.** For the game the AI added a "narcolepsy" goroutine that re-sleeps patients while HTTP handlers read and mutate the same hospital, but initially let it call the model without the interface lock. I route every handler *and* every tick of the goroutine through the `game` mutex, so the background wakes and the player's requests can never touch the model at the same time (still without changing the `hospital` package).
- **The client trusting its own optimistic state.** The first game build marked a patient awake locally and only logged a failed `POST`, so the UI and the server could drift apart. I revert the patient and roll the score back if the request fails, and re-sync from `/api/ward` on a timer so the server stays the single source of truth.

## Learning Summary
- Embedding `Person` in structs requires careful method receiver selection (pointer vs value)
- Go interfaces are satisfied implicitly, just call the methods and you are part of it
- A bidirectional association must be owned by one side (`Room.Occupy`/`Release` maintain the patient's back-reference) so the two objects cannot disagree.
- Validate in the constructor and return an error, so an invalid object can never exist; one shared sentinel with wrapped context beats a sentinel per field.
- Store a record of what happened by value (a snapshot), not by pointer to a still-mutable object.
- `atomic.AddInt64(&seq, 1)` is the idiomatic Go way for thread-safe counters
- An interface belongs outside the model: the `web` package drives the hospital only through its exported API, so the interface can be added, replaced or removed without touching the domain.
- When an interface adds its own goroutine, the same `sync.Mutex` that guards the handlers must also guard the goroutine, so the model is only ever touched under one lock.
- A front end that updates optimistically must undo the change on failure and resync from the server; the server, not the animation, is the source of truth.
