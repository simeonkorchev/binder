# The rate limiter's concurrent spec failed once under the parallel gate

- **Category**: flaky-test
- **Severity**: medium
- **Path**: `pkg/ygoprodeck/ratelimit_test.go` (the `callers arrive concurrently` spec)
- **Found**: 2026-09-18, during T048's local end-to-end verification

## What happened

One `make check` run failed:

```
[FAIL] RateLimiter when callers arrive concurrently
       [It] gives each one its own slot instead of letting them all through
FAIL! -- 23 Passed | 1 Failed
--- FAIL: TestYgoprodeck (0.04s)
```

I did not touch `pkg/ygoprodeck`. The very next `make check` was green, and the
gate has been green on every run since.

## What I could not do: reproduce it

| Attempt | Result |
|---|---|
| `go test -count=1 ./pkg/ygoprodeck/` × 12 | 12 pass |
| `go test -race -count=1` × 8 | 8 pass |
| `go test -race -count=1` × 6 with four `yes` processes saturating the CPU | 6 pass |

26 attempts, no failure. It only appeared inside the full gate, which runs every
suite in parallel under `-race`.

## The suspect (WRONG — see the update below)

The spec starts four goroutines, each calling `limiter.Wait(ctx)`, waits on a
`sync.WaitGroup`, then asserts:

```go
Expect(clock.sleeps()).To(HaveLen(callers - 1))
…
Expect(clock.Now().Sub(start)).To(Equal(callers * defaultInterval))
```

The comment above it is honest that "which goroutine gets which slot is a race"
and asserts only the count. So the count is what became wrong — which points at
the **fake clock**, not the limiter: if `sleeps()` and whatever `Wait` calls to
record a sleep are not mutually excluded, a reader can observe a slice mid-append
even after `wg.Wait()` has returned, because the append and the WaitGroup's
`Done` are not ordered with respect to each other unless the clock itself
synchronises.

That is a hypothesis. I did not confirm it, and I am not fixing it on a guess:
a speculative change to a test that already passes 26 times out of 26 would be
unverifiable either way.

## Proposed fix

Read `pkg/ygoprodeck`'s fake clock (the `clock` in the suite) and check whether
`sleeps()` and its recording path share a mutex. If they do not, that is the
bug, and the fix is a lock around both — in the fake, not in the limiter.

Then prove it: run the concurrent spec under `-race` in a loop with
`-count=200`, which is the shape that would surface an unsynchronised append,
rather than the whole-suite runs above.

If the fake is already synchronised, the next question is whether
`clock.Now().Sub(start)` can be read before the last paced caller has advanced
it, which would make the count assertion the symptom rather than the cause.

## Why it matters even though it is rare

CI has never run (`.github/workflows/ci.yml` is written but unexecuted). A spec
that fails roughly once in thirty parallel runs will go red on a pull request
eventually, and the first time it does, the person looking at it will not know
it is this. `.claude/rules/006-testing.md` treats a flake as a defect, not
weather.

---

## Update 2026-09-18: my hypothesis was wrong

I said the fake clock probably lacked a mutex. **It does not.** `pkg/ygoprodeck/ratelimit_test.go`:

```go
type fakeClock struct { mu sync.Mutex; now time.Time; slept []time.Duration }
```

`Now`, `Sleep` and `sleeps` all take `mu`, and `sleeps` returns a **copy**. There
is no unsynchronised append. That lead is dead — do not spend time on it.

`RateLimiter.reserve` is also sound: it holds `l.mu` while it reads the clock and
advances `l.next`, so four concurrent callers claim four distinct slots. Holding
the lock across the sleep is deliberately avoided, and the comment says why.

### What I now think, and could still not prove

The fake models a **shared** virtual clock and `Sleep` advances it by the
sleeper's *full* duration:

```go
c.slept = append(c.slept, d)
c.now = c.now.Add(d)
```

Real concurrent sleeps overlap: three goroutines sleeping 1i, 2i and 3i take 3i
of wall time. This fake makes them **accumulate** to 6i. Whether the spec's final
assertion — `clock.Now().Sub(start) == callers * defaultInterval` — holds
therefore depends on whether each caller's reserve/sleep pair interleaves one at
a time, which is exactly the scheduling the spec's own comment admits is a race.

So the assertion is only order-independent for the *lucky* interleaving. That
fits the symptom (rare, load-dependent, count assertion of 3 still satisfiable)
better than anything else I looked at.

**I could not reproduce it.** I wrote a harness that puts four goroutines behind
a barrier and releases them at once; the sleeps still came out `[50ms 50ms 50ms]`
— the interleaved order — and the elapsed time still matched. Reserving is
unexported, so forcing "all four reserve, then all four sleep" through the public
`Wait` is not something I found a way to do from outside the package.

### What the next person should try

An in-package test (`package ygoprodeck`, not `ygoprodeck_test`) can call
`reserve()` directly and construct the unlucky order deterministically: reserve
four times, *then* sleep the four returned durations. If elapsed comes out
`6 * interval` rather than `4 * interval`, this is confirmed and the fix is the
spec's, not the limiter's — assert the order-independent property (four slots
were consumed) instead of a wall-clock total a shared fake cannot model.

If that comes out right, the cause is still open and the next suspect is
whatever else in the parallel gate can perturb this suite.

**The limiter itself is not implicated by anything I found.** Nothing here
suggests production pacing is wrong; the doubt is about what the spec asserts.
