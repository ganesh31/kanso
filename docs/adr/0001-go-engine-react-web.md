# Go for the engine and API, React + TypeScript for the web app

Kanso targets full-stack product roles in fintech and is also the author's own trading tool. We chose a Go engine and API with a React + TypeScript web app instead of TypeScript end-to-end or Kotlin. Indian fintech backends run a lot of Go, goroutines parallelise scans across hundreds of instruments, the server ships as one static binary, and the author already knows Go. The cost is two languages, so the API types are bridged by code generation.

## Considered Options

- **TypeScript end-to-end** (Node/Bun or Next.js): the strongest "full-stack product" signal, with shared types for free. Rejected because a hand-written backtest engine in Go reads as more serious to fintech interviewers, scans would be limited to a single thread, and the plan would need a full rewrite.
- **Kotlin (Ktor)**: suits JVM fintech, but has a heavier runtime and build and is less common in Indian startup fintech.
