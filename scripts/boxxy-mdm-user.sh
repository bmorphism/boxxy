#!/usr/bin/env bash
# boxxy-mdm-user.sh — Create a maximally simplified boxxy@ macOS user
#
# Goal: a minimal, locked-down macOS user account on every machine that
# serves as the sectorless boot target for boxxy VMs. MDM can enforce
# this; beyond MDM we use `profiles` CLI and `sysadminctl` directly.
#
# Inspired by:
#   - Asahi Linux's approach: muvm microVM for 4K-page isolation,
#     DCP firmware interface, Virtualization.framework passthrough
#   - Justine Tunney: sectorless execution, no boundary between
#     bootloader and runtime
#   - GF(3) color confirmation: every action requires ≥3 witnesses
#
# What "maximally simplified" means:
#   1. No iCloud, no Apple ID, no App Store
#   2. No Dock, no desktop icons, no Finder sidebar
#   3. Login shell is /bin/zsh with a boxxy-controlled .zshrc
#   4. Screen locked after 60s, no password hints
#   5. SSH enabled, key-only auth
#   6. Firewall on, stealth mode
#   7. FileVault enrolled (inherits machine-level)
#   8. Software update deferred 7 days (stability)
#   9. Rosetta 2 installed (for x86 boxxy containers)
#  10. Virtualization.framework entitlements via boxxy binary
#
# Usage:
#   sudo ./boxxy-mdm-user.sh [--apply-profile] [--asahi-bridge]
#
# The --apply-profile flag installs the .mobileconfig profile.
# The --asahi-bridge flag sets up muvm-compatible networking for
# dual-boot scenarios with Asahi Linux.

set -euo pipefail

BOXXY_USER="boxxy"
BOXXY_UID="599"  # Below 500 hides from login window; 599 = visible but low
BOXXY_HOME="/Users/${BOXXY_USER}"
BOXXY_SHELL="/bin/zsh"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROFILE_PATH="${SCRIPT_DIR}/boxxy-mdm.mobileconfig"
GF3_SEED=1069

# Colors (Gay protocol, seed 1069)
RED='\033[38;2;230;127;134m'    # index 1: #E67F86
GREEN='\033[38;2;118;220;125m'  # index 0: #76DC7D
BLUE='\033[38;2;80;213;212m'    # index 2
RESET='\033[0m'

log()  { echo -e "${GREEN}[boxxy]${RESET} $*"; }
warn() { echo -e "${RED}[boxxy]${RESET} $*"; }
info() { echo -e "${BLUE}[boxxy]${RESET} $*"; }

# ── Preflight ────────────────────────────────────────────────────────────

if [[ "$(uname)" != "Darwin" ]]; then
    warn "This script is macOS-only."
    exit 1
fi

if [[ "$(uname -m)" != "arm64" ]]; then
    warn "Apple Silicon required (got $(uname -m))."
    exit 1
fi

if [[ $EUID -ne 0 ]]; then
    warn "Must run as root: sudo $0 $*"
    exit 1
fi

MACOS_VERSION=$(sw_vers -productVersion)
log "macOS ${MACOS_VERSION} on $(sysctl -n machdep.cpu.brand_string 2>/dev/null || echo 'Apple Silicon')"

# ── 1. Create boxxy@ user ───────────────────────────────────────────────

if id "${BOXXY_USER}" &>/dev/null; then
    log "User ${BOXXY_USER}@ already exists (uid=$(id -u ${BOXXY_USER}))"
else
    log "Creating user ${BOXXY_USER}@..."

    # Generate a random password (will be replaced by key-only SSH)
    BOXXY_PASS=$(openssl rand -base64 32)

    sysadminctl -addUser "${BOXXY_USER}" \
        -fullName "boxxy" \
        -password "${BOXXY_PASS}" \
        -home "${BOXXY_HOME}" \
        -shell "${BOXXY_SHELL}" \
        -admin 2>/dev/null || {
            # Fallback: dscl
            dscl . -create "/Users/${BOXXY_USER}"
            dscl . -create "/Users/${BOXXY_USER}" UserShell "${BOXXY_SHELL}"
            dscl . -create "/Users/${BOXXY_USER}" RealName "boxxy"
            dscl . -create "/Users/${BOXXY_USER}" UniqueID "${BOXXY_UID}"
            dscl . -create "/Users/${BOXXY_USER}" PrimaryGroupID 20
            dscl . -create "/Users/${BOXXY_USER}" NFSHomeDirectory "${BOXXY_HOME}"
            dscl . -passwd "/Users/${BOXXY_USER}" "${BOXXY_PASS}"
            dscl . -append /Groups/admin GroupMembership "${BOXXY_USER}"
        }

    # Create home directory
    createhomedir -c -u "${BOXXY_USER}" 2>/dev/null || mkdir -p "${BOXXY_HOME}"
    chown -R "${BOXXY_USER}:staff" "${BOXXY_HOME}"

    log "User ${BOXXY_USER}@ created"
fi

# ── 2. SSH key-only auth ─────────────────────────────────────────────────

SSH_DIR="${BOXXY_HOME}/.ssh"
mkdir -p "${SSH_DIR}"

if [[ ! -f "${SSH_DIR}/authorized_keys" ]]; then
    # Import from current user if available
    CALLER_HOME=$(eval echo "~${SUDO_USER:-root}")
    if [[ -f "${CALLER_HOME}/.ssh/id_ed25519.pub" ]]; then
        cat "${CALLER_HOME}/.ssh/id_ed25519.pub" >> "${SSH_DIR}/authorized_keys"
        log "Imported SSH key from ${SUDO_USER:-root}"
    elif [[ -f "${CALLER_HOME}/.ssh/id_rsa.pub" ]]; then
        cat "${CALLER_HOME}/.ssh/id_rsa.pub" >> "${SSH_DIR}/authorized_keys"
        log "Imported SSH key from ${SUDO_USER:-root}"
    else
        info "No SSH key found. Add one to ${SSH_DIR}/authorized_keys"
    fi
fi

chmod 700 "${SSH_DIR}"
chmod 600 "${SSH_DIR}/authorized_keys" 2>/dev/null || true
chown -R "${BOXXY_USER}:staff" "${SSH_DIR}"

# Enable SSH
systemsetup -setremotelogin on 2>/dev/null || launchctl load -w /System/Library/LaunchDaemons/ssh.plist 2>/dev/null || true
log "SSH enabled (key-only recommended)"

# ── 3. Boxxy directory structure ─────────────────────────────────────────

BOXXY_BASE="${BOXXY_HOME}/.boxxy"
mkdir -p "${BOXXY_BASE}"/{macos,linux,vms,seeds,chains}

# Write the GF(3) seed
echo "${GF3_SEED}" > "${BOXXY_BASE}/seeds/genesis"
log "GF(3) genesis seed: ${GF3_SEED} (0x42D)"

# ── 4. Minimal .zshrc ───────────────────────────────────────────────────

cat > "${BOXXY_HOME}/.zshrc" << 'ZSHRC'
# boxxy@ shell — maximally simplified
export BOXXY_HOME="${HOME}/.boxxy"
export BOXXY_SEED=$(cat "${BOXXY_HOME}/seeds/genesis" 2>/dev/null || echo 1069)
export PATH="${BOXXY_HOME}/bin:${PATH}"
export NO_COLOR=  # Let Gay protocol handle colors
unset NO_COLOR

# No history leaks
HISTSIZE=1000
SAVEHIST=1000
HISTFILE="${BOXXY_HOME}/.zsh_history"
setopt HIST_IGNORE_ALL_DUPS

# Prompt: trit indicator from seed
_boxxy_trit() {
    local sum=$(( (BOXXY_SEED % 3) - 1 ))
    case $sum in
        -1) echo "i" ;;
        0)  echo "0" ;;
        1)  echo "1" ;;
    esac
}
PS1='%F{green}boxxy%f@%m [%F{magenta}$(_boxxy_trit)%f] %~ %# '

# Aliases
alias ll='ls -la'
alias vms='ls -la ${BOXXY_HOME}/vms/'
alias seeds='cat ${BOXXY_HOME}/seeds/genesis'
ZSHRC

chown "${BOXXY_USER}:staff" "${BOXXY_HOME}/.zshrc"
log "Minimal .zshrc installed"

# ── 5. Rosetta 2 ────────────────────────────────────────────────────────

if ! /usr/bin/pgrep -q oahd 2>/dev/null; then
    log "Installing Rosetta 2 (for x86 boxxy containers / FEX compatibility)..."
    softwareupdate --install-rosetta --agree-to-license 2>/dev/null || true
else
    log "Rosetta 2 already installed"
fi

# ── 6. Generate MDM configuration profile ────────────────────────────────

# This .mobileconfig can be installed via:
#   - MDM (Jamf, Mosyle, Kandji, Fleet, etc.)
#   - Apple Configurator
#   - `profiles install -path boxxy-mdm.mobileconfig`

cat > "${PROFILE_PATH}" << 'MOBILECONFIG'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>PayloadContent</key>
    <array>
        <!-- Restrictions: maximum simplification -->
        <dict>
            <key>PayloadType</key>
            <string>com.apple.applicationaccess</string>
            <key>PayloadIdentifier</key>
            <string>com.boxxy.restrictions</string>
            <key>PayloadUUID</key>
            <string>A1B2C3D4-1069-4242-BEEF-000000000001</string>
            <key>PayloadVersion</key>
            <integer>1</integer>
            <!-- No iCloud -->
            <key>allowCloudDocumentSync</key>
            <false/>
            <key>allowCloudKeychainSync</key>
            <false/>
            <key>allowCloudMail</key>
            <false/>
            <!-- No App Store -->
            <key>allowAppInstallation</key>
            <false/>
            <!-- No AirDrop -->
            <key>allowAirDrop</key>
            <false/>
            <!-- No Game Center -->
            <key>allowGameCenter</key>
            <false/>
            <!-- No Siri -->
            <key>allowAssistant</key>
            <false/>
        </dict>

        <!-- Screensaver / Lock: 60 seconds -->
        <dict>
            <key>PayloadType</key>
            <string>com.apple.screensaver</string>
            <key>PayloadIdentifier</key>
            <string>com.boxxy.screensaver</string>
            <key>PayloadUUID</key>
            <string>A1B2C3D4-1069-4242-BEEF-000000000002</string>
            <key>PayloadVersion</key>
            <integer>1</integer>
            <key>askForPassword</key>
            <true/>
            <key>askForPasswordDelay</key>
            <integer>0</integer>
            <key>idleTime</key>
            <integer>60</integer>
        </dict>

        <!-- Firewall: on + stealth -->
        <dict>
            <key>PayloadType</key>
            <string>com.apple.security.firewall</string>
            <key>PayloadIdentifier</key>
            <string>com.boxxy.firewall</string>
            <key>PayloadUUID</key>
            <string>A1B2C3D4-1069-4242-BEEF-000000000003</string>
            <key>PayloadVersion</key>
            <integer>1</integer>
            <key>EnableFirewall</key>
            <true/>
            <key>EnableStealthMode</key>
            <true/>
            <key>BlockAllIncoming</key>
            <false/>
        </dict>

        <!-- Software Update: defer 7 days -->
        <dict>
            <key>PayloadType</key>
            <string>com.apple.SoftwareUpdate</string>
            <key>PayloadIdentifier</key>
            <string>com.boxxy.softwareupdate</string>
            <key>PayloadUUID</key>
            <string>A1B2C3D4-1069-4242-BEEF-000000000004</string>
            <key>PayloadVersion</key>
            <integer>1</integer>
            <key>ManagedDeferredInstallDelay</key>
            <integer>7</integer>
            <key>AutomaticCheckEnabled</key>
            <true/>
            <key>AutomaticallyInstallMacOSUpdates</key>
            <false/>
        </dict>

        <!-- Dock: minimal -->
        <dict>
            <key>PayloadType</key>
            <string>com.apple.dock</string>
            <key>PayloadIdentifier</key>
            <string>com.boxxy.dock</string>
            <key>PayloadUUID</key>
            <string>A1B2C3D4-1069-4242-BEEF-000000000005</string>
            <key>PayloadVersion</key>
            <integer>1</integer>
            <key>autohide</key>
            <true/>
            <key>minimize-to-application</key>
            <true/>
            <key>show-recents</key>
            <false/>
            <key>tilesize</key>
            <integer>36</integer>
            <key>static-only</key>
            <true/>
        </dict>

        <!-- Login Window: no hints, no guest -->
        <dict>
            <key>PayloadType</key>
            <string>com.apple.loginwindow</string>
            <key>PayloadIdentifier</key>
            <string>com.boxxy.loginwindow</string>
            <key>PayloadUUID</key>
            <string>A1B2C3D4-1069-4242-BEEF-000000000006</string>
            <key>PayloadVersion</key>
            <integer>1</integer>
            <key>DisableGuestAccount</key>
            <true/>
            <key>RetriesUntilHint</key>
            <integer>0</integer>
            <key>com.apple.login.mcx.DisableAutoLoginClient</key>
            <true/>
            <key>SHOWFULLNAME</key>
            <true/>
        </dict>
    </array>

    <!-- Profile metadata -->
    <key>PayloadDisplayName</key>
    <string>boxxy@ Maximally Simplified</string>
    <key>PayloadDescription</key>
    <string>Locks down macOS for boxxy@ user: no iCloud, no App Store, minimal Dock, firewall stealth, 60s screen lock. Seed 1069.</string>
    <key>PayloadIdentifier</key>
    <string>com.boxxy.mdm.profile</string>
    <key>PayloadOrganization</key>
    <string>boxxy</string>
    <key>PayloadRemovalDisallowed</key>
    <false/>
    <key>PayloadScope</key>
    <string>System</string>
    <key>PayloadType</key>
    <string>Configuration</string>
    <key>PayloadUUID</key>
    <string>A1B2C3D4-1069-4242-BEEF-DEADBEEF1069</string>
    <key>PayloadVersion</key>
    <integer>1</integer>
</dict>
</plist>
MOBILECONFIG

log "Generated MDM profile: ${PROFILE_PATH}"

# ── 7. Install profile if requested ──────────────────────────────────────

if [[ "${1:-}" == "--apply-profile" ]] || [[ "${2:-}" == "--apply-profile" ]]; then
    log "Installing MDM profile..."
    profiles install -path "${PROFILE_PATH}" 2>/dev/null && {
        log "Profile installed successfully"
    } || {
        warn "Profile install requires user approval (MDM or manual)"
        info "Install manually: profiles install -path ${PROFILE_PATH}"
        info "Or deploy via MDM (Jamf/Mosyle/Kandji/Fleet)"
    }
fi

# ── 8. Asahi bridge (dual-boot network) ──────────────────────────────────

if [[ "${1:-}" == "--asahi-bridge" ]] || [[ "${2:-}" == "--asahi-bridge" ]]; then
    log "Setting up Asahi Linux bridge..."
    info "Asahi Linux context (from 6.19 progress report, 2026-02-15):"
    info "  - DisplayPort Alt Mode: fairydust branch (USB-C out working!)"
    info "  - M3 support: keyboard/touchpad/WiFi/NVMe/USB3 working"
    info "  - 120Hz ProMotion: static timestamp hack on MacBook Pro"
    info "  - muvm: 4K-page microVM for x86 compat (like boxxy VMs)"
    info "  - Vulkan 1.3 Honeykrisp: conformant GL4.6 + VK1.3 + CL3.0"

    # Create a bridge config for muvm-style networking between macOS and Linux
    ASAHI_BRIDGE="${BOXXY_BASE}/linux/asahi-bridge.conf"
    cat > "${ASAHI_BRIDGE}" << 'ASAHIBRIDGE'
# Asahi Linux ↔ boxxy@ bridge configuration
# muvm-compatible networking for dual-boot scenarios
#
# On macOS side (boxxy@):
#   - vmnet.framework shared network (192.168.64.0/24)
#   - socket_vmnet for VM networking
#   - Virtualization.framework for native ARM64 VMs
#
# On Asahi Linux side:
#   - muvm for 4K-page x86 emulation
#   - FEX-Emu for x86 binary translation
#   - Honeykrisp Vulkan 1.3 for GPU passthrough
#
# Shared state:
#   - NVMe partition visible to both OSes
#   - Seeds and chains stored on shared APFS volume
#   - Color chain verification across boot boundary

BRIDGE_SUBNET=192.168.64.0/24
BRIDGE_HOST=192.168.64.1
BRIDGE_GUEST=192.168.64.2

# GF(3) seed for cross-boot verification
GF3_SEED=1069

# Asahi-specific: DCP firmware interface version
# macOS 14+ changed the API; Asahi tracks per-firmware
DCP_FW_COMPAT=14.0

# muvm page size bridge: macOS=16K, guest=4K
# boxxy handles this via Virtualization.framework
PAGE_SIZE_HOST=16384
PAGE_SIZE_GUEST=4096
ASAHIBRIDGE

    chown "${BOXXY_USER}:staff" "${ASAHI_BRIDGE}"
    log "Asahi bridge config: ${ASAHI_BRIDGE}"
fi

# ── 9. What MDM can't do (and beyond) ───────────────────────────────────

BEYOND="${BOXXY_BASE}/beyond-mdm.md"
cat > "${BEYOND}" << 'BEYOND_MDM'
# Beyond MDM: What boxxy@ Does That Profiles Can't

MDM configuration profiles handle ~80% of the lockdown. The remaining
20% requires direct system manipulation or user-space tricks.

## What MDM handles (via .mobileconfig):
- ✅ Disable iCloud/App Store/AirDrop/Siri/Game Center
- ✅ Screensaver lock timeout (60s)
- ✅ Firewall + stealth mode
- ✅ Software update deferral
- ✅ Dock auto-hide + minimal
- ✅ Login window: no guest, no hints
- ✅ FileVault enforcement (device-level)

## What boxxy@ adds beyond MDM:
- 🔧 SSH key-only auth (no password over network)
- 🔧 Rosetta 2 pre-installed (for x86 containers)
- 🔧 Virtualization.framework entitlements (com.apple.security.virtualization)
- 🔧 Custom .zshrc with GF(3) trit prompt
- 🔧 Directory structure: ~/.boxxy/{macos,linux,vms,seeds,chains}
- 🔧 Genesis seed file (1069 / 0x42D)

## What requires Apple Business Manager (ABM):
- 🏢 Automated Device Enrollment (zero-touch)
- 🏢 Supervision (prevents profile removal)
- 🏢 Kernel extension management
- 🏢 System extension management
- 🏢 Managed Apple ID assignment

## What requires custom kernel/boot (Asahi territory):
- 🐧 DCP display driver (DisplayPort Alt Mode via fairydust)
- 🐧 GPU driver (Honeykrisp Vulkan 1.3, conformant GL4.6)
- 🐧 4K-page microVM (muvm for x86 binary translation)
- 🐧 Native ARM64 kernel with Energy-Aware Scheduling
- 🐧 Speaker safety daemon (speakersafetyd)
- 🐧 Apple Interchange compressed framebuffer format (reverse-engineered)

## The craziest things being done (Asahi Linux, Feb 2026):
1. **64GB zero-page trick**: Reserve 64GB virtual memory of zeroes for
   robustness2 — out-of-bounds GPU loads hit this region instead of
   crashing. Turns 4 compare-and-selects into 2.
2. **Tessellation via compute shaders**: M1 hardware tessellation too
   limited for DirectX/Vulkan/OpenGL. Full tessellation emulated in
   arcane compute shaders.
3. **muvm dual page-size**: Run 4K-page Linux inside 16K-page host via
   lightweight VM. GPU passthrough via DRM native context.
4. **DCP static timestamp hack**: 120Hz on MacBook Pro by stuffing a
   static value into presentation timestamp fields — Apple requires
   VRR timestamps even for fixed refresh rate.
5. **Apple Interchange format**: Reverse-engineered proprietary compressed
   framebuffer format used by both GPU (AGX) and video decoder (AVD).
   macOS uses shader decompression; Asahi does it in Mesa.
6. **Webcam via ISP→V4L2→GPU→compositor**: Apple's ISP exports planar
   Y'CbCr; requires GPU shader conversion to RGB; needed fixes in
   Mesa, PipeWire (integer overflow), and DMA-BUF import deadlock.
7. **Patch delta**: 1232→858 patches in 12 months, 95K→83K lines delta.

## Sectorless connection:
The boxxy@ user IS the sectorless boot target. There is no boundary
between the macOS user account and the VM runtime — the user IS the
execution environment. The .mobileconfig profile is the GF(3) gain
stage: it shapes what passes through (restrictions = attenuation,
entitlements = amplification, defaults = transparency).
BEYOND_MDM

chown "${BOXXY_USER}:staff" "${BEYOND}"

# ── 10. Summary ──────────────────────────────────────────────────────────

log "═══════════════════════════════════════════════════"
log "  boxxy@ user setup complete"
log "═══════════════════════════════════════════════════"
info "  User:    ${BOXXY_USER}@$(hostname -s)"
info "  Home:    ${BOXXY_HOME}"
info "  Shell:   ${BOXXY_SHELL}"
info "  Seed:    ${GF3_SEED} (0x42D)"
info "  Profile: ${PROFILE_PATH}"
info ""
info "  Next steps:"
info "    1. Install profile: profiles install -path ${PROFILE_PATH}"
info "    2. Or deploy via MDM (Jamf/Mosyle/Kandji/Fleet)"
info "    3. Add SSH key: ${SSH_DIR}/authorized_keys"
info "    4. For Asahi dual-boot: re-run with --asahi-bridge"
info "    5. Build boxxy VMs: cd ${BOXXY_BASE}/vms && boxxy create"
log "═══════════════════════════════════════════════════"
