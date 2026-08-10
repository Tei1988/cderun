# /proc/self/mountinfo Specification

The `/proc/self/mountinfo` file in Linux systems provides detailed information about mount points within the mount namespace currently seen by the process.
Especially when debugging or analyzing container technologies (such as Docker and Kubernetes) or complex bind mount situations, it is highly useful because it yields more precise and detailed information than `/proc/mounts`.

## 1. Overview

This file maintains mount information on a per-process basis in a line-oriented text format. Each line represents a single mount point, with fields separated by whitespace (spaces).

## 2. Data Structure Format

The data format per line is as follows:

```text
36 35 98:0 / /mnt rw,noatime master:1 - ext3 /dev/root rw,errors=continue
(1)(2)(3) (4) (5)     (6)      (7)   (8) (9)    (10)         (11)
```

## 3. Detailed Field Definitions

The meaning of each field is as follows:

| No. | Field Name | Data Type | Description | Example |
| :-- | :--- | :--- | :--- | :--- |
| 1 | Mount ID | Integer | A unique ID for this mount. | 36 |
| 2 | Parent ID | Integer | The ID of the parent mount (its own ID if it is the root). | 35 |
| 3 | Major:Minor | String | The major and minor device numbers of the file system. | 98:0 |
| 4 | Root | Path | The root directory within the mounted file system. | / or /var/lib |
| 5 | Mount Point | Path | The mount point relative to the process's root. | /mnt |
| 6 | Mount Options | String | Per-mount options. | rw,noatime |
| 7 | Optional Tags | String | Mount propagation settings, etc. (detailed below). | shared:1, master:1 |
| 8 | Separator | Char | A separator indicating the end of the optional fields. | - |
| 9 | FS Type | String | The file system type. | ext3, tmpfs, nfs |
| 10 | Mount Source | String | File system-specific mount source information. | /dev/sda1 |
| 11 | Superblock | String | Per-superblock options. | rw,errors=continue |

## 4. Special Notes: Optional Tags (Field 7)

Field 7 is variable-length and may contain the following keywords. These are primarily related to mount **propagation**:

- **shared:X**: The mount is shared within peer group X. Events (mount/unmount) propagate to other mount points in the same group.
- **master:X**: The mount is a slave to peer group X, receiving propagation from the master, but not vice versa.
- **propagate_from:X**: The propagation source group when the mount is both a slave and shared.
- **unbindable**: This mount point cannot be further bind-mounted.

*Note: If this field is absent, it means the mount is private (does not propagate events to/from other mounts).*

## 5. Differences from /proc/mounts

Compared to the traditional `/proc/mounts` (or `/etc/mtab`), `mountinfo` provides the following benefits:

- **Unique IDs (Mount ID, Parent ID)**: Accurately tracks parent-child relationships of mount points and the sequence when stacked (overlay) mounts are applied on the same path.
- **Root Path Identification (Root)**: For bind mounts, it determines whether the mount is the "entire root of the file system" or just a "specific subdirectory" (Field 4).
- **Propagation Attribute Verification**: Only `mountinfo` allows verifying critical container-environment attributes like shared or slave.

## 6. Parsing Considerations

- **Variable-Length Field 7**: When writing a parser (analysis program), do not treat fields as fixed columns; you must find the separator (`-` / Field 8) first, and identify the subsequent fields (FS Type, etc.) relative to it.
- **Escape Processing**: Any spaces, newlines, or other special characters in path names are escaped in octal format (e.g., `\040` for spaces).

Based on this information, you can investigate which host directory a specific container mounts, or diagnose why a mount has unexpectedly become read-only (whether due to mount options or superblock options).
