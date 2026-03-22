# TODO

## Container Runtime
- Improve image pull rate limit handling (e.g., configurable retry strategies or registry-specific backoff).

## Testing & Maintenance
- Investigate missing coverage for `SignalContainer` failure and `AttachTimeout` in `internal/command/root.go` (requires complex mocking of internal signals and constants).

## Environment & CI
- Investigate recurring Podman test failures in CI (Podman v5.8.1). The tests fail with "Cannot connect to the Docker daemon at unix:///run/podman/podman.sock" or "EOF" during image pull, despite the socket being accessible according to diagnostics.
  - Failure Example: `TestScenario_Execution_AlpineEcho/echo_hello` fails with `failed to pull image: failed to inspect image: Cannot connect to the Docker daemon at unix:///run/podman/podman.sock`.
  - Diagnostics show: `name: podman`, `socket: /run/podman/podman.sock`, `status: accessible`.
  - **Constraints & Suggestions**:
    - **前回の修正でタイムアウトの時間を延ばしたり、リトライ回数を増やしたりする実装は誤りであったので、その変更は禁止です。**
    - **まずはGithubActions周りの変更や、Podmanのランタイムが利用されるようになっているかのチェックをしてください。**
