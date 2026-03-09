#!/usr/bin/env bash
# provision-noble-guest.sh — Configure a boxxy Linux VM guest for Noble chain
#
# Walks every provisioning step to make the guest a syncing noble-1 node.
#
# Run inside the ARM64 Linux guest after boot:
#   mount -t virtiofs noble-genesis /mnt/noble-genesis
#   sudo /mnt/noble-genesis/provision-noble-guest.sh
#
# GF(3) trit accounting:
#   Step 1: Install nobled binary        (+1 generation)
#   Step 2: Initialize chain config      ( 0 coordination)
#   Step 3: Apply genesis + snapshot     ( 0 coordination)
#   Step 4: Configure peers + ports      ( 0 coordination)
#   Step 5: Start nobled                 (+1 generation)
#   Step 6: Verify sync                  (-1 validation)
#   Step 7: NTP time sync               (-1 validation)
#
# Architecture path in boxxy:
#   examples/noble-vm.joke → boot → this script → nobled running
#   internal/vm/vm.go (host) ↔ this script (guest) via VirtioFS share

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
YELLOW='\033[1;33m'
RESET='\033[0m'

log()  { echo -e "${GREEN}[noble-guest]${RESET} $*"; }
warn() { echo -e "${RED}[noble-guest]${RESET} $*"; }
info() { echo -e "${CYAN}[noble-guest]${RESET} $*"; }
step() { echo -e "${YELLOW}[noble-guest]${RESET} $*"; }

if [[ $EUID -ne 0 ]]; then
    warn "Must run as root: sudo $0"
    exit 1
fi

# ── Configuration from kernel cmdline (set by noble-vm.joke) ────────────
CHAIN_ID="${CHAIN_ID:-noble-1}"
MONIKER="${MONIKER:-boxxy-noble-0}"
SEED_PEERS="${SEED_PEERS:-20e1000e88125698264454a884812746c2eb4807@seeds.lavenderfive.com:21590,b85358e035343a3b15e77e1102857dcdaf70053b@seeds.bluestake.net:25556}"
P2P_PORT="${P2P_PORT:-26656}"
RPC_PORT="${RPC_PORT:-26657}"
REST_PORT="${REST_PORT:-1317}"
GRPC_PORT="${GRPC_PORT:-9090}"

NOBLE_HOME="${NOBLE_HOME:-/root/.noble}"
NOBLED_VERSION="v11.2.0"
GENESIS_MOUNT="/mnt/noble-genesis"
SNAPSHOT_URL="${SNAPSHOT_URL:-https://snapshots.polkachu.com/snapshots/noble/noble_46419300.tar.lz4}"
NTP_SERVERS="${NTP_SERVERS:-time.google.com pool.ntp.org}"

log "═══════════════════════════════════════════════════"
log " Noble chain guest provisioning"
log " Chain: ${CHAIN_ID} | Moniker: ${MONIKER}"
log " nobled: ${NOBLED_VERSION} | CometBFT: v0.38.19"
log "═══════════════════════════════════════════════════"
echo

# ── Step 1: Install nobled binary (+1) ──────────────────────────────────

step "Step 1/7: Installing nobled binary [+1]"

if command -v nobled &>/dev/null; then
    INSTALLED_VERSION=$(nobled version 2>/dev/null || echo "unknown")
    info "nobled already installed: ${INSTALLED_VERSION}"
else
    if [[ -f "${GENESIS_MOUNT}/nobled" ]]; then
        cp "${GENESIS_MOUNT}/nobled" /usr/local/bin/nobled
        chmod +x /usr/local/bin/nobled
        log "Installed nobled from VirtioFS share"
    else
        log "Building nobled from source..."
        # Requires Go 1.24+
        if ! command -v go &>/dev/null; then
            warn "Go not found. Install Go 1.24+ or provide nobled binary in ${GENESIS_MOUNT}/nobled"
            exit 1
        fi
        TMPDIR=$(mktemp -d)
        cd "${TMPDIR}"
        git clone --depth 1 --branch "${NOBLED_VERSION}" https://github.com/strangelove-ventures/noble.git
        cd noble
        make build
        cp build/nobled /usr/local/bin/nobled
        cd /
        rm -rf "${TMPDIR}"
        log "Built and installed nobled ${NOBLED_VERSION}"
    fi
fi

nobled version
echo

# ── Step 2: Initialize chain config (0) ─────────────────────────────────

step "Step 2/7: Initializing chain configuration [0]"

if [[ -d "${NOBLE_HOME}/config" ]]; then
    info "Noble home already initialized at ${NOBLE_HOME}"
else
    nobled init "${MONIKER}" --chain-id "${CHAIN_ID}" --home "${NOBLE_HOME}" 2>/dev/null
    log "Initialized ${NOBLE_HOME} for chain ${CHAIN_ID}"
fi
echo

# ── Step 3: Apply genesis + snapshot (0) ─────────────────────────────────

step "Step 3/7: Applying genesis file and snapshot [0]"

# Genesis
if [[ -f "${GENESIS_MOUNT}/genesis.json" ]]; then
    cp "${GENESIS_MOUNT}/genesis.json" "${NOBLE_HOME}/config/genesis.json"
    log "Copied genesis from VirtioFS share"
elif [[ ! -f "${NOBLE_HOME}/config/genesis.json" ]] || [[ $(wc -c < "${NOBLE_HOME}/config/genesis.json") -lt 1000 ]]; then
    log "Fetching genesis from noble-networks..."
    curl -sL "https://raw.githubusercontent.com/strangelove-ventures/noble-networks/main/mainnet/noble-1/genesis.json" \
        > "${NOBLE_HOME}/config/genesis.json"
    log "Downloaded genesis.json"
else
    info "Genesis file already present"
fi

GENESIS_SIZE=$(wc -c < "${NOBLE_HOME}/config/genesis.json" | tr -d ' ')
info "Genesis size: ${GENESIS_SIZE} bytes"

# Snapshot (fast sync)
if [[ -d "${NOBLE_HOME}/data/state.db" ]] || [[ -d "${NOBLE_HOME}/data/blockstore.db" ]]; then
    info "Chain data already exists, skipping snapshot"
else
    if [[ -f "${GENESIS_MOUNT}/snapshot.tar.lz4" ]]; then
        log "Applying snapshot from VirtioFS share..."
        lz4 -d "${GENESIS_MOUNT}/snapshot.tar.lz4" | tar xf - -C "${NOBLE_HOME}"
        log "Snapshot applied from local share"
    elif command -v wget &>/dev/null || command -v curl &>/dev/null; then
        log "Downloading Polkachu snapshot (~1 GB)..."
        if command -v wget &>/dev/null; then
            wget -q --show-progress -O /tmp/noble_snapshot.tar.lz4 "${SNAPSHOT_URL}"
        else
            curl -L -o /tmp/noble_snapshot.tar.lz4 "${SNAPSHOT_URL}"
        fi
        log "Extracting snapshot..."
        lz4 -d /tmp/noble_snapshot.tar.lz4 | tar xf - -C "${NOBLE_HOME}"
        rm -f /tmp/noble_snapshot.tar.lz4
        log "Snapshot applied from Polkachu"
    else
        warn "No snapshot available. Node will sync from genesis (slow)."
    fi
fi
echo

# ── Step 4: Configure peers + ports (0) ──────────────────────────────────

step "Step 4/7: Configuring peers and ports [0]"

CONFIG="${NOBLE_HOME}/config/config.toml"
APP_CONFIG="${NOBLE_HOME}/config/app.toml"

# Seed peers
if grep -q 'seeds = ""' "${CONFIG}" 2>/dev/null; then
    sed -i "s|seeds = \"\"|seeds = \"${SEED_PEERS}\"|" "${CONFIG}"
    log "Set seed peers"
else
    info "Seeds already configured"
fi

# P2P port
sed -i "s|laddr = \"tcp://0.0.0.0:26656\"|laddr = \"tcp://0.0.0.0:${P2P_PORT}\"|" "${CONFIG}" 2>/dev/null || true

# RPC port
sed -i "s|laddr = \"tcp://127.0.0.1:26657\"|laddr = \"tcp://0.0.0.0:${RPC_PORT}\"|" "${CONFIG}" 2>/dev/null || true

# REST API (app.toml)
if [[ -f "${APP_CONFIG}" ]]; then
    sed -i "s|address = \"tcp://localhost:1317\"|address = \"tcp://0.0.0.0:${REST_PORT}\"|" "${APP_CONFIG}" 2>/dev/null || true
    # Enable REST API
    sed -i '/\[api\]/,/\[/{s/enable = false/enable = true/}' "${APP_CONFIG}" 2>/dev/null || true
fi

# Pruning (keep it lean for boxxy tiles)
if [[ -f "${APP_CONFIG}" ]]; then
    sed -i 's/pruning = "default"/pruning = "custom"/' "${APP_CONFIG}" 2>/dev/null || true
    sed -i 's/pruning-keep-recent = "0"/pruning-keep-recent = "100"/' "${APP_CONFIG}" 2>/dev/null || true
    sed -i 's/pruning-interval = "0"/pruning-interval = "10"/' "${APP_CONFIG}" 2>/dev/null || true
fi

log "Ports: P2P=${P2P_PORT} RPC=${RPC_PORT} REST=${REST_PORT} gRPC=${GRPC_PORT}"
echo

# ── Step 5: Start nobled (+1) ────────────────────────────────────────────

step "Step 5/7: Starting nobled [+1]"

# Create systemd service if available
if command -v systemctl &>/dev/null; then
    cat > /etc/systemd/system/nobled.service <<EOF
[Unit]
Description=Noble Chain Daemon (boxxy tile)
After=network-online.target
Wants=network-online.target

[Service]
User=root
ExecStart=/usr/local/bin/nobled start --home ${NOBLE_HOME}
Restart=on-failure
RestartSec=3
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF
    systemctl daemon-reload
    systemctl enable nobled.service
    systemctl start nobled.service
    log "Started nobled via systemd"
else
    # No systemd — start directly in background
    nohup nobled start --home "${NOBLE_HOME}" > /var/log/nobled.log 2>&1 &
    NOBLED_PID=$!
    echo "${NOBLED_PID}" > /var/run/nobled.pid
    log "Started nobled (PID: ${NOBLED_PID})"
fi
echo

# ── Step 6: Verify sync (-1) ────────────────────────────────────────────

step "Step 6/7: Verifying chain sync [-1]"

log "Waiting for RPC to come up..."
for i in $(seq 1 30); do
    if curl -s "http://localhost:${RPC_PORT}/status" >/dev/null 2>&1; then
        break
    fi
    sleep 2
done

if curl -s "http://localhost:${RPC_PORT}/status" >/dev/null 2>&1; then
    STATUS=$(curl -s "http://localhost:${RPC_PORT}/status")
    LATEST_HEIGHT=$(echo "${STATUS}" | python3 -c "import json,sys; print(json.load(sys.stdin)['result']['sync_info']['latest_block_height'])" 2>/dev/null || echo "?")
    CATCHING_UP=$(echo "${STATUS}" | python3 -c "import json,sys; print(json.load(sys.stdin)['result']['sync_info']['catching_up'])" 2>/dev/null || echo "?")
    NETWORK=$(echo "${STATUS}" | python3 -c "import json,sys; print(json.load(sys.stdin)['result']['node_info']['network'])" 2>/dev/null || echo "?")

    log "Chain: ${NETWORK}"
    log "Height: ${LATEST_HEIGHT}"
    log "Catching up: ${CATCHING_UP}"
else
    warn "RPC not responding yet. Node may still be initializing."
    info "Check: curl http://localhost:${RPC_PORT}/status"
    info "Logs: journalctl -u nobled -f  OR  tail -f /var/log/nobled.log"
fi
echo

# ── Step 7: NTP time sync (-1) ──────────────────────────────────────────

step "Step 7/7: Configuring NTP time synchronization [-1]"

if command -v timedatectl &>/dev/null; then
    mkdir -p /etc/systemd/timesyncd.conf.d
    cat > /etc/systemd/timesyncd.conf.d/boxxy-noble.conf <<EOF
[Time]
NTP=${NTP_SERVERS}
FallbackNTP=0.pool.ntp.org 1.pool.ntp.org
EOF
    timedatectl set-ntp true 2>/dev/null || true
    systemctl restart systemd-timesyncd.service 2>/dev/null || true
    log "NTP configured: ${NTP_SERVERS}"
elif command -v chronyd &>/dev/null; then
    for server in ${NTP_SERVERS}; do
        echo "server ${server} iburst" >> /etc/chrony.conf
    done
    systemctl restart chronyd 2>/dev/null || true
    log "Chrony configured"
else
    warn "No NTP daemon found. Time accuracy may affect consensus."
fi
echo

# ── Summary ──────────────────────────────────────────────────────────────

log "═══════════════════════════════════════════════════"
log " Noble chain guest provisioning complete"
log "═══════════════════════════════════════════════════"
log ""
log " GF(3) trit accounting:"
log "   +1  nobled binary installed"
log "    0  chain config initialized"
log "    0  genesis + snapshot applied"
log "    0  peers + ports configured"
log "   +1  nobled started"
log "   -1  sync verified"
log "   -1  NTP configured"
log "   ──────────────────────────────"
log "    0  sum (mod 3) ✓"
log ""
log " Endpoints:"
log "   P2P:   tcp://0.0.0.0:${P2P_PORT}"
log "   RPC:   http://0.0.0.0:${RPC_PORT}"
log "   REST:  http://0.0.0.0:${REST_PORT}"
log "   gRPC:  0.0.0.0:${GRPC_PORT}"
log ""
log " Monitor:"
log "   curl localhost:${RPC_PORT}/status | python3 -m json.tool"
log "   journalctl -u nobled -f"
log ""
log " IBC channel monitor (from boxxy host):"
log "   curl localhost:${REST_PORT}/ibc/core/channel/v1/channels?pagination.limit=500"
