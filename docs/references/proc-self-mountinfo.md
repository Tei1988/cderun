# /proc/self/mountinfo Specification

The `/proc/self/mountinfo` file in Linux systems provides detailed information about mount points within the mount namespace currently seen by the process.
Especially when debugging or analyzing container technologies (such as Docker and Kubernetes) or complex bind mount situations, it is highly useful because it yields more precise and detailed information than `/proc/mounts`.

## 1. Overview

This file maintains mount information on a per-process basis in a line-oriented text format. Each line represents a single mount point, with fields separated by whitespace (spaces).

## 2. Data Structure Format

The data format per line consists of several fixed-position fields, a variable list of optional fields, a separator, and post-separator fields:

```text
36 35 98:0 / /mnt rw,noatime master:1 - ext3 /dev/root rw,errors=continue
(1)(2)(3) (4) (5)     (6)      (7)   (8) (9)    (10)         (11)
```

*Note: The numbers in parentheses above show a conceptual layout, but fields following the optional tags are not at fixed positions.*

## 3. Detailed Field Definitions

The meaning of each field is defined as follows:

| Field Group | Field Name | Data Type | Description | Example |
| :--- | :--- | :--- | :--- | :--- |
| **Fixed Fields** | Mount ID | Integer | A unique ID for this mount. Uniqueness applies only within a single namespace snapshot; IDs may be reused after unmounting, so they are not stable identities across successive snapshots. | 36 |
| | Parent ID | Integer | The ID of the parent mount (its own ID if it is the root). | 35 |
| | Major:Minor | String | The major and minor device numbers of the file system. | 98:0 |
| | Root | Path | The root directory within the mounted file system. | / or /var/lib |
| | Mount Point | Path | The mount point relative to the process's root. | /mnt |
| | Mount Options | String | Per-mount options. | rw,noatime |
| **Optional Fields** | Optional Tags | String | Zero or more space-delimited `tag[:value]` tokens (such as `shared:1` or `master:1`). Unknown optional tags are ignored. | master:1 |
| **Separator** | Separator | Char | A single hyphen (`-`) indicating the end of the optional fields. | - |
| **Post-Separator Fields** | FS Type | String | The file system type (parsed after locating the separator). | ext3 |
| | Mount Source | String | File system-specific mount source information. | /dev/root |
| | Superblock | String | Per-superblock options. | rw,errors=continue |

## 4. Special Notes: Optional Tags

The optional tags field is variable-length, space-delimited, and may contain the following keywords. These are primarily related to mount **propagation**:

- **shared:X**: The mount is shared within peer group X. Events (mount/unmount) propagate to other mount points in the same group.
- **master:X**: The mount is a slave to peer group X, receiving propagation from the master, but not vice versa.
- **propagate_from:X**: Describes the nearest reachable dominant peer group for a slave mount. It may appear alongside `master:X` to designate propagation ancestry, rather than indicating that the mount is both slave and shared.
- **unbindable**: This mount point cannot be further bind-mounted.

*Note: If no optional tags are present, the mount is private (does not propagate events to/from other mounts).*

## 5. Differences from /proc/mounts

Compared to the traditional `/proc/mounts` (or `/etc/mtab`), `mountinfo` provides the following benefits:

- **Unique IDs (Mount ID, Parent ID)**: Accurately tracks parent-child relationships of mount points and the sequence when stacked (overlay) mounts are applied on the same path.
- **Root Path Identification (Root)**: For bind mounts, it determines whether the mount is the "entire root of the file system" or just a "specific subdirectory" (e.g. `/var/lib`).
- **Propagation Attribute Verification**: Only `mountinfo` allows verifying critical container-environment attributes like shared or slave.

## 6. Parsing Considerations

- **Variable-Length Optional Tags**: When writing a parser (analysis program), do not treat post-optional fields (like FS Type) as sitting at fixed column indices. You must find the separator (`-`) first, and identify the subsequent fields (FS Type, Mount Source, and Superblock options) relative to it.
- **Escape Processing**: Any spaces, newlines, or other special characters in path names are escaped in octal format (e.g., `\040` for spaces).

Based on this information, you can investigate mounts visible in the current mount namespace. Note that these entries describe namespace-local states and do not guarantee the existence of a corresponding host-side directory. Correctly mapping a container mount back to a host directory requires additional container or runtime context. However, mountinfo remains highly valuable for diagnosing why a mount has unexpectedly become read-only (whether due to mount options or superblock options).
