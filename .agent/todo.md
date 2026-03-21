# TODO
## Container Runtime

### Podman (v5.8.1)
- Podman runtime tests are failing in CI with `Cannot connect to the Docker daemon at unix:///run/podman/podman.sock`.
- Diagnosis indicates that the socket is accessible, but `Post "http://%2Frun%2Fpodman%2Fpodman.sock/v1.44/images/create?fromImage=public.ecr.aws%2Fdocker%2Flibrary%2Falpine&tag=latest": EOF` error occurs during image pull.
- This suggests a potential compatibility issue with the Podman API version or the way `cderun` interacts with the Podman service.
- [ ] Investigate intermittent Podman connection failures in CI ("Cannot connect to the Docker daemon at unix:///run/podman/podman.sock"). Observed during internal/config test improvement task.
- [ ] 
