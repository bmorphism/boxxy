#!/usr/bin/env bash
# provision-linux-guest.sh — Configure a boxxy Linux VM guest
#
# Registers Rosetta as the binfmt_misc handler for x86_64 ELF binaries
# and configures systemd-timesyncd for NTP synchronization.
#
# Run inside the ARM64 Linux guest after boot:
#   sudo ./provision-linux-guest.sh
#
# Requires:
#   - Apple Silicon host with Rosetta for Linux enabled
#   - Rosetta VirtioFS share mounted (tag "rosetta")
#   - systemd-based distribution (Arch, Ubuntu, Guix System w/ shepherd alternative)

set -euo pipefail

# ── GF(3) trit accounting ───────────────────────────────────────────────
# Step 1: Mount Rosetta share       (+1 generation)
# Step 2: Register binfmt handler   ( 0 coordination)
# Step 3: Sync time via NTP         (-1 validation)
# Sum = 0 (mod 3)

RED='\033[0;31m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
RESET='\033[0m'

log()  { echo -e "${GREEN}[boxxy-guest]${RESET} $*"; }
warn() { echo -e "${RED}[boxxy-guest]${RESET} $*"; }
info() { echo -e "${CYAN}[boxxy-guest]${RESET} $*"; }

if [[ $EUID -ne 0 ]]; then
    warn "Must run as root: sudo $0"
    exit 1
fi

NTP_SERVERS="${BOXXY_NTP_SERVERS:-time.google.com pool.ntp.org}"
ROSETTA_TAG="${BOXXY_ROSETTA_TAG:-rosetta}"
ROSETTA_MOUNTPOINT="/mnt/rosetta"

# ── 1. Mount Rosetta VirtioFS share (+1) ────────────────────────────────

log "Step 1/3: Mounting Rosetta directory share [+1]"

mkdir -p "${ROSETTA_MOUNTPOINT}"

if mountpoint -q "${ROSETTA_MOUNTPOINT}" 2>/dev/null; then
    info "Rosetta share already mounted at ${ROSETTA_MOUNTPOINT}"
else
    if mount -t virtiofs "${ROSETTA_TAG}" "${ROSETTA_MOUNTPOINT}" 2>/dev/null; then
        log "Mounted virtiofs tag '${ROSETTA_TAG}' at ${ROSETTA_MOUNTPOINT}"
    else
        warn "Failed to mount Rosetta VirtioFS share."
        warn "Ensure the host VM config has EnableRosetta=true and tag='${ROSETTA_TAG}'"
        warn "Continuing without Rosetta (x86_64 binaries will not run)."
    fi
fi

# Persist the mount in fstab if not already present
if ! grep -q "${ROSETTA_TAG}" /etc/fstab 2>/dev/null; then
    echo "${ROSETTA_TAG} ${ROSETTA_MOUNTPOINT} virtiofs ro,nofail 0 0" >> /etc/fstab
    log "Added Rosetta mount to /etc/fstab"
fi

# ── 2. Register binfmt_misc handler for x86_64 ELF (0) ─────────────────

log "Step 2/3: Registering Rosetta as x86_64 ELF handler [0]"

ROSETTA_BIN="${ROSETTA_MOUNTPOINT}/rosetta"

if [[ ! -x "${ROSETTA_BIN}" ]]; then
    warn "Rosetta binary not found at ${ROSETTA_BIN}"
    warn "Skipping binfmt registration."
else
    # Ensure binfmt_misc is mounted
    if ! mountpoint -q /proc/sys/fs/binfmt_misc 2>/dev/null; then
        mount -t binfmt_misc binfmt_misc /proc/sys/fs/binfmt_misc 2>/dev/null || true
    fi

    if command -v update-binfmts &>/dev/null; then
        # Debian/Ubuntu style
        if update-binfmts --display rosetta 2>/dev/null | grep -q "enabled"; then
            info "Rosetta binfmt handler already registered"
        else
            update-binfmts \
                --install rosetta \
                "${ROSETTA_BIN}" \
                --magic '\x7fELF\x02\x01\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x02\x00\x3e\x00' \
                --mask '\xff\xff\xff\xff\xff\xff\xff\x00\xff\xff\xff\xff\xff\xff\xff\xff\xfe\xff\xff\xff' \
                --credentials yes \
                --fix-binary yes
            log "Registered Rosetta via update-binfmts"
        fi
    else
        # Direct binfmt_misc registration (Arch, Guix, etc.)
        #
        # Format: :name:type:offset:magic:mask:interpreter:flags
        #   F = fix-binary (resolve interpreter at registration time)
        #   C = credentials (run with binary's credentials, not interpreter's)
        #
        # Magic (20 bytes):  7f454c46 02010100 0000000000000000 02003e00
        # Mask  (20 bytes):  ffffffff ffffff00 ffffffffffffffff feffffff
        #
        # Byte 7  mask=00: wildcard OS/ABI (accepts SYSV=0x00 and GNU=0x03)
        # Byte 16 mask=fe: accepts ET_EXEC=0x02 and ET_DYN=0x03 (PIE binaries)
        # Bytes 18-19:      EM_X86_64 = 0x003e = 62

        BINFMT_ENTRY=":rosetta:M::\\x7fELF\\x02\\x01\\x01\\x00\\x00\\x00\\x00\\x00\\x00\\x00\\x00\\x00\\x02\\x00\\x3e\\x00:\\xff\\xff\\xff\\xff\\xff\\xff\\xff\\x00\\xff\\xff\\xff\\xff\\xff\\xff\\xff\\xff\\xfe\\xff\\xff\\xff:${ROSETTA_BIN}:FC"

        if [[ -f /proc/sys/fs/binfmt_misc/rosetta ]]; then
            info "Rosetta binfmt handler already registered"
        else
            echo "${BINFMT_ENTRY}" > /proc/sys/fs/binfmt_misc/register
            log "Registered Rosetta via /proc/sys/fs/binfmt_misc/register"
        fi

        # Persist via systemd-binfmt if available
        if [[ -d /etc/binfmt.d ]]; then
            echo "${BINFMT_ENTRY}" > /etc/binfmt.d/rosetta.conf
            log "Persisted binfmt config to /etc/binfmt.d/rosetta.conf"
        fi
    fi

    # Verify
    if [[ -f /proc/sys/fs/binfmt_misc/rosetta ]]; then
        log "Verification: $(head -1 /proc/sys/fs/binfmt_misc/rosetta)"
    fi
fi

# ── 3. Configure systemd-timesyncd (-1) ─────────────────────────────────

log "Step 3/3: Configuring NTP time synchronization [-1]"

if command -v timedatectl &>/dev/null; then
    # systemd-timesyncd path (Arch, Ubuntu, most systemd distros)

    # Write config drop-in (doesn't clobber distro defaults)
    mkdir -p /etc/systemd/timesyncd.conf.d
    cat > /etc/systemd/timesyncd.conf.d/boxxy.conf <<EOF
[Time]
NTP=${NTP_SERVERS}
FallbackNTP=0.pool.ntp.org 1.pool.ntp.org 2.pool.ntp.org 3.pool.ntp.org
RootDistanceMaxSec=5
PollIntervalMinSec=32
PollIntervalMaxSec=2048
EOF
    log "Wrote /etc/systemd/timesyncd.conf.d/boxxy.conf"
    info "NTP servers: ${NTP_SERVERS}"

    # Enable and restart
    timedatectl set-ntp true 2>/dev/null || \
        systemctl enable --now systemd-timesyncd.service 2>/dev/null || true

    systemctl restart systemd-timesyncd.service 2>/dev/null || true

    # Wait briefly for initial sync
    sleep 2

    # Show status
    if timedatectl timesync-status &>/dev/null; then
        log "Time sync status:"
        timedatectl timesync-status 2>/dev/null | head -5 || true
    else
        timedatectl status 2>/dev/null | grep -E "NTP|synchronized|Time zone" || true
    fi

    # Verify with journal
    info "Recent timesyncd log:"
    journalctl -u systemd-timesyncd --no-hostname --since "1 minute ago" -n 3 2>/dev/null || true

elif command -v ntpd &>/dev/null; then
    # Fallback: traditional ntpd
    warn "systemd-timesyncd not found, using ntpd"
    for server in ${NTP_SERVERS}; do
        if ! grep -q "${server}" /etc/ntp.conf 2>/dev/null; then
            echo "server ${server} iburst" >> /etc/ntp.conf
        fi
    done
    systemctl restart ntpd 2>/dev/null || service ntpd restart 2>/dev/null || true

elif command -v chronyd &>/dev/null; then
    # Fallback: chrony
    warn "systemd-timesyncd not found, using chrony"
    for server in ${NTP_SERVERS}; do
        if ! grep -q "${server}" /etc/chrony.conf 2>/dev/null; then
            echo "server ${server} iburst" >> /etc/chrony.conf
        fi
    done
    systemctl restart chronyd 2>/dev/null || service chronyd restart 2>/dev/null || true

else
    warn "No NTP daemon found. Install systemd-timesyncd, chrony, or ntp."
    warn "Manual sync: ntpdate ${NTP_SERVERS%% *}"
fi

# ── Summary ─────────────────────────────────────────────────────────────

echo
log "═══════════════════════════════════════════════════"
log " boxxy Linux guest provisioning complete"
log "═══════════════════════════════════════════════════"
log ""
log " GF(3) trit accounting:"
log "   +1  Rosetta VirtioFS mount (generation)"
log "    0  binfmt_misc x86_64 handler (coordination)"
log "   -1  NTP time sync (validation)"
log "   ──────────────────────────────"
log "    0  sum (mod 3) -- invariant maintained"
log ""

if [[ -x "${ROSETTA_BIN}" ]]; then
    log " x86_64 support: ENABLED"
    log "   binary: ${ROSETTA_BIN}"
    log "   ELF magic: 7f454c46 02010100 (class64, LE, v1)"
    log "   mask:      ffffffff ffffff00 (any OS/ABI)"
    log "   types:     ET_EXEC + ET_DYN (static + PIE)"
    log "   machine:   EM_X86_64 (0x3e)"
    log "   flags:     F=fix-binary C=credentials"
else
    warn " x86_64 support: DISABLED (Rosetta not available)"
fi

echo
if command -v timedatectl &>/dev/null; then
    SYNCED=$(timedatectl show -p NTPSynchronized --value 2>/dev/null || echo "unknown")
    log " NTP sync: ${SYNCED}"
    log "   servers: ${NTP_SERVERS}"
    log "   config:  /etc/systemd/timesyncd.conf.d/boxxy.conf"
    log "   verify:  timedatectl timesync-status"
    log "   log:     journalctl -u systemd-timesyncd"
fi

echo
log " Next: test x86_64 binary execution"
log "   file /usr/bin/some-x86-binary  # confirm x86_64 ELF"
log "   /usr/bin/some-x86-binary       # should run via Rosetta"
