# TODO
## Test Improvement
### Introduce Podman in Docker in CI
**Objective:**
Expand our CI suite to ensure the CLI tool's compatibility with Podman v5.8.1, paralleling our current Docker in Docker (DinD) setup.

**Context:**
While our current DinD tests cover standard container workflows, Podman introduces unique behaviors (e.g., rootless execution, different volume mounting logic). We need to verify that our tool works seamlessly across both runtimes to prevent regressions.

**Target File:**
- .github/workflows/ci.yaml

**Key Requirements:**
1. **New Test Job:** Create a new job (e.g., test-podman) or extend the existing matrix to include Podman.
1. **Container Image:** Use quay.io/podman/stable:v5.8.1 as the execution environment.
1. **Environment Setup:**
  - Ensure the container runs in --privileged mode to allow Podman-to-Podman nesting.
  - Configure necessary environment variables or aliases (e.g., alias docker=podman) if the test suite expects a docker command.
1. **Validation:** All existing integration tests must pass within the Podman container.

**Success Criteria:**
The CI pipeline successfully triggers and passes a full test run using the Podman v5.8.1 image.
