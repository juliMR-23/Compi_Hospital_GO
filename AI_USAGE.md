# AI Usage Documentation

## Tools Used
- **Gemini**: Initial project scaffolding, struct design guidance, interface patterns
- **JetBrains AI assistant (GoLand)**: Code completion for getter methods and boilerplate

## Key Prompts
1. "How to handle inheritance (non-native in GO) with embedding"
2. "Help me use counters for automatic ID generation for every entity"
3. "How do you implement an interface in GO? Attender interface with Doctor and Orderly types"

## Lessons Learned - AI Generated Code Issues
- **Starvation from a trivially "always available" attender.** The initial `Dispatch` selected the first attender whose `IsAvailable()` returned `true`. Because `Orderly.IsAvailable()` always returns `true`, the orderly would have absorbed every callback and both doctors would have finished with zero episodes attended. **Human decision:** I added a `nextAttender` round-robin cursor on `Hospital` so dispatch walks the `staff []Attender` starting from the last used position and wraps around, keeping the rotation fair (the concrete `Doctor`/`Orderly` type is still chosen at runtime through the interface). I later simplified the model further: `Dispatch` is only for emergencies, so `IsAvailable` was removed from the `Attender` interface and now lives only on `Doctor`, where it gates `DiagnosePatient` (taking patients under care) and never attendance. This makes Query 5.2 meaningful and avoids one attender stalling the others.
- **Room capacity not enforced.** The first `Room.Occupy` draft simply appended the patient to `occupants` and flipped the state to `Occupied`, without checking the capacity or whether the patient was already inside. A capacity-1 room could therefore hold several patients and the same patient could take two slots, which also broke the derived `Occupied` state. **Human decision:** I made `NewRoom` reject `capacity <= 0` (`ErrInvalidInput`), made `Occupy` refuse overflow (`ErrRoomOccupied`) and duplicates (`ErrAlreadyInRoom`), and derived the state from `len(occupants)` via `refreshState` so a room stays `Available` until it is genuinely full.

## Learning Summary
- `atomic.AddInt64(&seq, 1)` is the idiomatic Go way for thread-safe counters
- Embedding `Person` in structs requires careful method receiver selection (pointer vs value)
- Go interfaces are satisfied implicitly, just call the methods and you are part of it