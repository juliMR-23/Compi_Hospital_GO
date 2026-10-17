# AI Usage Documentation

## Tools Used
- **Gemini**: Initial project scaffolding, struct design guidance, interface patterns
- **JetBrains AI assistant (GoLand)**: Code completion for getter methods and boilerplate

## Key Prompts
1. "How to handle inheritance (non-native in GO) with embedding"
2. "Help me use counters for automatic ID generation for every entity"
3. "How do you implement an interface in GO? Attender interface with Doctor and Orderly types"

## Lessons Learned - AI Generated Code Issues

## Learning Summary
- `atomic.AddInt64(&seq, 1)` is the idiomatic Go way for thread-safe counters
- Embedding `Person` in structs requires careful method receiver selection (pointer vs value)
- Go interfaces are satisfied implicitly, just call the methods and you are part of it