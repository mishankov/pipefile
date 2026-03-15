# Code Review: Uncommitted Changes

**Review Target**: Uncommitted changes on branch `cooking`  
**Change Type**: Major refactor (feature + refactor)  
**Date**: 2026-03-14

---

## Change Summary

Refactor from sequential polling-based execution to parallel dependency-based execution with DAG validation.

| File | Change |
|------|--------|
| `application/application.go` | Initialize model with context, handle init errors |
| `application/commands.go` | Rewrite: event-driven commands with `stepRunner` abstraction |
| `application/messages.go` | **DELETED** — consolidated into `commands.go` |
| `application/model.go` | Major rewrite: dependency graph, parallel execution, cancellation |
| `executor/executor.go` | Refactor: return errors, use `exec.CommandContext` |
| `pipefile.toml` | Test failure command `1/0` added to deploy step |
| `application/model_test.go` | **NEW** — comprehensive test coverage |
| `executor/executor_test.go` | **NEW** — executor tests |

Key Architectural Changes:
- Sequential execution → Parallel/dependency-based execution
- Polling (`executor.Finished` flag) → Event-driven (`tea.Cmd` pattern)
- No dependency validation → DAG with cycle detection
- No cancellation → Context-based graceful shutdown
- Thread-unsafe buffer → Mutex-protected `logBuffer`

---

## Findings

### P1: Unbounded `logBuffer` Memory Growth

**Impact**: Memory exhaustion, OOM crashes  
**Failure Mode**: Long-running commands (e.g., `docker build`, `mvn install`) produce megabytes of output stored indefinitely in memory.

**Evidence**:  
`application/model.go:32-53` — `logBuffer` wraps `bytes.Buffer` with mutex but no size limit.

```go
func (b *logBuffer) Write(p []byte) (int, error) {
    b.mu.Lock()
    defer b.mu.Unlock()
    return b.buf.Write(p)  // Unbounded growth
}
```

**Recommended Fix**:  
- Add ` maxSize int64` field to `logBuffer`
- Implement ring buffer or truncate to last N lines
- Alternatively, stream to file and render tail

---

### P1: Executor Swallows Command Error on Context Cancellation

**Impact**: Debugging difficulty, masked failures  
**Failure Mode**: When context is cancelled while a command is running, the original command error is discarded.

**Evidence**:  
`executor/executor.go:24-29`:
```go
err := cmd.Run()
if err != nil && ctx.Err() != nil {
    return ctx.Err()
}
return err
```

When both `err != nil` and `ctx.Err() != nil`, only `ctx.Err()` is returned, losing information about command failure.

**Recommended Fix**:
```go
err := cmd.Run()
if err != nil {
    if ctx.Err() != nil {
        return ctx.Err()
    }
    return err
}
return nil
```

---

### P1: Fragile Cancellation State Assumption

**Impact**: Incorrect step status display  
**Failure Mode**: Steps cancelled mid-flight show `error` instead of `canceled` if `pipelineFailed` not set before cancellation.

**Evidence**:  
`application/model.go:217-219`:
```go
case errors.Is(msg.err, context.Canceled) && m.pipelineFailed:
    current.status = stepStatusCanceled
default:
    current.status = stepStatusError
```

The logic requires `m.pipelineFailed == true` BEFORE the cancellation error arrives. If execution order varies, `canceled` status may not apply correctly.

**Recommended Fix**:  
Pass explicit cancellation reason through error chain, or use sentinel error to distinguish:
- User-initiated quit (`ctrl+c`)
- Pipeline failure propagation
- Step-level failure

---

### P2: Unused `finishedCount` Counter

**Impact**: Dead code, maintenance confusion  
**Evidence**:  
`application/model.go:199`:
```go
m.finishedCount++
```

Counter is incremented but never rendered or used in logic.

**Recommended Fix**:  
- Remove if truly unused
- Or implement progress display (e.g., "3/5 steps completed")

---

### P2: Missing Concurrent `logBuffer` Write Test

**Impact**: Untested concurrent code path  
**Evidence**:  
`application/model_test.go` — no test exercises parallel writes to same `logBuffer`.

When steps run in parallel (fan-out), multiple goroutines write to `outputBuff` concurrently. While mutex protects against races, this code path lacks explicit test coverage.

**Recommended Fix**:  
Add test:
```go
func TestLogBufferConcurrentWrites(t *testing.T) {
    var buf logBuffer
    var wg sync.WaitGroup
    
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for j := 0; j < 100; j++ {
                buf.Write([]byte("line\n"))
            }
        }()
    }
    
    wg.Wait()
    // Verify no data loss/corruption
}
```

---

### P2: `runningCount` Decrement Missing Guard

**Impact**: Negative counter potential  
**Evidence**:  
`application/model.go:200-202`:
```go
if m.runningCount > 0 {
    m.runningCount--
}
```

Guard exists. **No issue found.**

---

### P3: View Header Minor Enhancement

**Observation**:  
Viewport shows step logs clearly, but status line could show more context (e.g., running/stopped state, elapsed time).

**Recommendation**: Optional enhancement, not blocking.

---

## Testing Coverage

### Covered Scenarios
- ✅ Step validation (missing deps, duplicates, cycles)
- ✅ Linear dependency execution order
- ✅ Parallel fan-out (multiple steps after single dependency)
- ✅ Parallel fan-in (step waits for multiple dependencies)
- ✅ Failure blocks dependents
- ✅ Cancellation status propagation
- ✅ Navigation (left/right key handling)
- ✅ Executor stop-on-failure
- ✅ Executor context cancellation

### Missing Coverage
- ❌ Concurrent `logBuffer` writes
- ❌ Context cancellation stops running `exec.Command`
- ❌ Model rebuild after resize event
- ❌ Empty pipeline (no steps)

---

## Residual Risks

1. **Memory exhaustion** from unbounded log buffers (P1)
2. **State ordering** dependency between `pipelineFailed` and cancellation (P1)
3. **Error masking** in executor when context cancels (P1)

---

## Questions / Assumptions

1. ✅ `1/0` in `pipefile.toml` is intentional for testing failure handling
2. ❓ Is `finishedCount` intended for future progress display feature?
3. ❓ Should `logBuffer` have configurable max size?

---

## Verdict

**Recommendation**: Address P1 issues before merge. P2 issues can be addressed in follow-up.

The refactor is architecturally sound with excellent test coverage for core DAG execution logic. Main concerns are memory safety and error handling edge cases.