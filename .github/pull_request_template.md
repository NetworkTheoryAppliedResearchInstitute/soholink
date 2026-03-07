## Summary
<!-- 1–3 bullet points describing what changed and why -->
-

## Type of change
- [ ] Bug fix
- [ ] New feature / enhancement
- [ ] Refactor (no behaviour change)
- [ ] Documentation / tooling

## Test plan
<!-- How did you verify this works?  Check all that apply. -->
- [ ] `make test` passes locally
- [ ] `make audit` passes locally (`make lint` + `make deadcode` + wiring tests)
- [ ] Manual smoke test (describe below)

---

## Wiring checklist
<!-- Complete this section whenever you add, remove, or rename a package-level
     registration function (e.g. SetDefaultConfig, SetDefaultPolicy, NewEngine). -->

**Did you add a new package-level registration function (`SetXxx`, `RegisterXxx`)?**
- [ ] No — skip the rest of this section.
- [ ] Yes — complete the items below.

If yes:
- [ ] The function's godoc comment says `// must be called before app.New()` (or documents when it must be called).
- [ ] `cmd/soholink/main.go`: the new call is present in `main()` **and** documented in the `Startup wiring` comment block.
- [ ] `internal/audit/wiring_test.go`: the symbol string is added to `requiredMainCalls`.
- [ ] `make audit` passes with the new symbol present.

**Did you remove or rename a registration function?**
- [ ] No — skip.
- [ ] Yes:
  - [ ] All call sites in `cmd/*/main.go` are updated.
  - [ ] `internal/audit/wiring_test.go` `requiredMainCalls` is updated.
  - [ ] `make audit` passes.

**Did you add a new subsystem that `app.New()` must initialise?**
- [ ] No — skip.
- [ ] Yes:
  - [ ] `internal/app/app.go`: subsystem is wired in `New()`.
  - [ ] `internal/app/startup_test.go` (or equivalent): a test exercises `app.New()` with the new subsystem active.
  - [ ] `make test` passes.

---

## Notes for reviewer
<!-- Anything else the reviewer should know — trade-offs, follow-ups, etc. -->
