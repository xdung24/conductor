<!-- avoid unnecessary back and forth: fill out every section below before submitting. -->

## Summary

<!-- Required: describe what this PR does and why. Replace this comment — PRs without a summary will be closed. -->

## Type of change

- [ ] Bug fix
- [ ] New feature
- [ ] Refactor / code quality
- [ ] Dependency update
- [ ] Documentation

## Related issues

<!-- Link any related issues below. Remove lines that don't apply. -->
- Relates to #
- Resolves #

## Changes made

<!-- List the specific files / components changed and what was done. Example:
- `internal/models/store.go` — replaced Exec with ExecContext throughout
- `internal/monitor/checker.go` — added new checker for X
-->

## Checklist

- [ ] `go build ./...` passes with no errors
- [ ] `go vet ./...` passes with no warnings
- [ ] `go test ./...` passes (or new tests added for new logic)
- [ ] Error return values are checked — no unchecked `errcheck` violations
- [ ] All DB calls use `ExecContext` / `QueryContext` / `QueryRowContext`, never the bare variants
- [ ] New migration files follow the `NNNN_description.up.sql` / `.down.sql` naming convention and existing files were not edited
- [ ] `FEATURES.md` updated if a planned/in-progress feature was completed
- [ ] No sensitive data (passwords, tokens, secrets) introduced in code or templates
