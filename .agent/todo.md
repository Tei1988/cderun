# TODO

## Container Runtime
- [x] Improve Podman CI stability by addressing connection resets (EOF) and API versioning issues.
- [x] Refactor container runtime initialization to cleanly separate Docker and Podman configurations.
- [x] Implement exponential backoff for image pulls with a configurable or sane base (1s).
- [x] Optimize error matching in `isRetryablePullError` by removing redundant checks and expanding connection error keywords.
