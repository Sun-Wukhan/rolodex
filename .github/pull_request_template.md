## Summary

<!-- What changed and why. PR title must follow Conventional Commits (feat:, fix:, chore:, ...). -->

## Test plan

- [ ] `make ci` (gofmt, vet, golangci-lint, tests with coverage gate, govulncheck)
- [ ] `cd frontend && npm run lint && npm run typecheck && npm run test:coverage`
- [ ] New behaviour is covered by unit tests

## Security checklist

- [ ] No secrets or credentials committed
- [ ] User input is validated at the service layer
- [ ] Database access stays inside `backend/internal/repository`
