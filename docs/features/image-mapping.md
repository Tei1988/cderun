# Feature Specification: Image Mapping

## Overview

`cderun` automatically maps subcommands to appropriate container images, eliminating the need to specify the container image name manually every time.

## Configuration

Image mapping configuration is defined in `.tools.yaml`:

```yaml
# .tools.yaml
node:
  image: "node:18-alpine"
python:
  image: "python:3.11-slim"
custom-tool:
  image: "my-registry/custom:1.2.3"
```

### Error Handling

- If no image mapping exists for a specified subcommand, execution terminates immediately with an error.
- Example: `cderun unknown-tool` -> `Error: no image mapping found for tool: unknown-tool`
- To bypass this lookup and run arbitrary tools, you must specify the image explicitly using the `--image` flag.

## Benefits

- **Convenience**: No need to memorize complex container image tags.
- **Consistency**: Standardized tool execution environments across machines.
- **Flexibility**: Custom mapping support via per-project or global configuration profiles.
