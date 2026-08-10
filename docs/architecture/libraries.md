# Libraries and Technology Stack

This document describes the technology stack and selection criteria for libraries used in this project.
When introducing a new external library, you must obtain user permission before running `go get`.

---

## 1. Core Technology

- **Language:** Go (latest stable version)
- **Module Management:** Go Modules (`go.mod`)

---

## 2. Selection Criteria

When deciding which library to select, use the following priority order:

1. **Go Standard Library:** Can this be achieved using only standard packages? (to minimize dependencies)
2. **Simplicity:** Is the library over-engineered (Overkill) for the required feature?
3. **Community:** Are the GitHub star count, maintenance frequency, and documentation quality sufficient?

---

## 3. Approved Libraries

The primary libraries whose usage is currently approved in the project:

- **CLI Framework:** [cobra](https://github.com/spf13/cobra)
- **Container Runtime API:** [moby (Docker)](https://github.com/moby/moby), [containerd/containerd/v2](https://github.com/containerd/containerd), [containerd/errdefs](https://github.com/containerd/errdefs) (introduced for containerd runtime support)
- **OCI Specification:** [opencontainers/image-spec](https://github.com/opencontainers/image-spec), [opencontainers/runtime-spec](https://github.com/opencontainers/runtime-spec)
- **YAML & Config Utilities:** [yaml.v3](https://gopkg.in/yaml.v3), [mergo](https://dario.cat/mergo)
- **Utilities:** [uuid](https://github.com/google/uuid), [go-units](https://github.com/docker/go-units), [go-connections](https://github.com/docker/go-connections), [x/term](https://golang.org/x/term)
- **Testing:** [testify](https://github.com/stretchr/testify)
