# Bolt's Performance Journal

## 2026-02-27 - Fast-path and Caching for Configuration Expressions
**Learning:** In CLI tools that resolve many configuration fields, redundant regular expression matching and repetitive disk I/O for file-based expressions (`{{ file:filename }}`) can significantly impact startup time. Adding a simple `strings.Contains` fast-path for expression detection and a per-resolver cache for file content provides a measurable performance boost.
**Action:** Always check for the presence of expression markers before invoking regex engines, and cache resolved file contents when the resolution state (e.g., current directory) is stable during the resolution cycle.
