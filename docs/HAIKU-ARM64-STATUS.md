# HaikuOS ARM64 on Apple Silicon via boxxy

## Current Status (January 2026)

### Summary
**HaikuOS ARM64 cannot currently run on Apple Silicon via boxxy** because:
1. No pre-built ARM64 images exist
2. The ARM64 port is only ~30% complete (kernel in progress, no app server)
3. Building from source on macOS fails due to toolchain incompatibilities

### Port Status (from haiku-os.org)

| Platform | Loader | Kernel | App Server | Status |
|----------|--------|--------|------------|--------|
| QEMU (EFI) | ✅ Complete | 🔄 In Progress | ❌ No Work | 30% |
| Raspberry Pi 4+ | 🔄 In Progress | ❌ No Work | ❌ No Work | 10% |

### What Works with boxxy

✅ **Alpine Linux aarch64** - Fully functional with GUI
✅ **FreeBSD aarch64** - Should work (has complete ARM64 port)
✅ **Other ARM64 Linux distros** - Ubuntu, Fedora, etc.

### Paths Forward

#### Option 1: Wait for HaikuOS ARM64 Development
The Haiku ARM64 port is being actively developed. Check:
- https://download.haiku-os.org/nightly-images/arm64/ (when nightlies appear)
- https://www.haiku-os.org/guides/building/port_status/

#### Option 2: Build from Linux
Building Haiku ARM64 requires a Linux host due to macOS toolchain issues.

```bash
# In a Linux environment (e.g., UTM/Docker):
git clone https://review.haiku-os.org/haiku
git clone https://review.haiku-os.org/buildtools

cd haiku
mkdir generated.arm64 && cd generated.arm64
../configure --cross-tools-source ../../buildtools --build-cross-tools arm64

# Build minimal image
jam -j$(nproc) -q @minimum-mmc
```

The resulting `haiku-mmc.image` could then be used with boxxy.

#### Option 3: Use UTM/QEMU for x86_64 HaikuOS
For a working HaikuOS experience today, use:
- **UTM** (free, macOS native) with x86_64 emulation
- Download: https://www.haiku-os.org/get-haiku/r1beta5/

#### Option 4: Contribute to HaikuOS ARM64 Port
The Haiku project welcomes contributors. Key areas needing work:
- Kernel completion (interrupt handling, scheduling, etc.)
- Device drivers (virtio for QEMU)
- Application Server port

## Technical Details

### Why x86_64 HaikuOS Doesn't Work
Apple Virtualization.framework only supports native ARM64 guests. 
It cannot emulate x86_64 - that requires QEMU userspace emulation.

### Why Rosetta Doesn't Help
Rosetta for Virtualization.framework is specifically for **Linux x86_64 binaries 
running inside an ARM64 Linux VM**. It:
- Requires an ARM64 Linux kernel as the host OS
- Only translates ELF x86_64 → ARM64 at the binary level
- Cannot run non-Linux operating systems (HaikuOS uses different ABIs)

### Build Failure on macOS
```
error: expected ')' in fdopen macro expansion
```
This is caused by Haiku's buildtools using old-style K&R function declarations
that conflict with modern Clang headers on macOS 15+.

## Files

- Build source (case-sensitive volume): `/Volumes/HaikuBuild/`
- Sparse image: `~/Downloads/haiku-build.sparseimage`

## Related boxxy Examples

- `cmd/alpine-gui/main.go` - Working ARM64 Linux with GUI
- `cmd/haiku-gui/main.go` - Ready for HaikuOS ARM64 (when available)
