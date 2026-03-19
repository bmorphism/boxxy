#!/bin/bash
# boxxy-pfctl-helper.sh — Minimal sudo surface for pf pinhole management.
# This is the ONLY file that requires sudo privileges.
#
# Usage:
#   sudo boxxy-pfctl-helper.sh activate <conf-path> <anchor-name>
#   sudo boxxy-pfctl-helper.sh deactivate <anchor-name>
#   sudo boxxy-pfctl-helper.sh status <anchor-name>
#
# GF(3) Trit: 0 (Coordinator) — mediates access policy via pf anchors

set -euo pipefail

usage() {
    echo "usage: boxxy-pfctl-helper.sh <activate|deactivate|status> [args...]" >&2
    echo "" >&2
    echo "  activate <conf-path> <anchor>   Load pf rules from conf into anchor" >&2
    echo "  deactivate <anchor>             Flush anchor rules" >&2
    echo "  status <anchor>                 Show current anchor rules" >&2
    exit 1
}

validate_anchor() {
    local anchor="$1"
    # Only allow com.boxxy.* anchors for safety
    if [[ ! "$anchor" =~ ^com\.boxxy\. ]]; then
        echo "error: anchor must start with com.boxxy. (got: $anchor)" >&2
        exit 1
    fi
}

validate_conf() {
    local conf="$1"
    # Only allow /tmp/boxxy-pinhole-* paths
    if [[ ! "$conf" =~ ^/tmp/boxxy-pinhole- ]]; then
        echo "error: conf must be in /tmp/boxxy-pinhole-* (got: $conf)" >&2
        exit 1
    fi
    if [ ! -f "$conf" ]; then
        echo "error: conf file does not exist: $conf" >&2
        exit 1
    fi
}

cmd_activate() {
    if [ $# -lt 2 ]; then
        echo "error: activate requires <conf-path> <anchor>" >&2
        exit 1
    fi
    local conf="$1"
    local anchor="$2"

    validate_anchor "$anchor"
    validate_conf "$conf"

    # Load rules into the anchor
    pfctl -a "$anchor" -f "$conf" 2>/dev/null
    echo "pinhole activated: anchor=$anchor conf=$conf"

    # Show loaded rules
    pfctl -a "$anchor" -sr 2>/dev/null
}

cmd_deactivate() {
    if [ $# -lt 1 ]; then
        echo "error: deactivate requires <anchor>" >&2
        exit 1
    fi
    local anchor="$1"

    validate_anchor "$anchor"

    # Flush anchor rules
    pfctl -a "$anchor" -F rules 2>/dev/null
    echo "pinhole deactivated: anchor=$anchor"
}

cmd_status() {
    if [ $# -lt 1 ]; then
        echo "error: status requires <anchor>" >&2
        exit 1
    fi
    local anchor="$1"

    validate_anchor "$anchor"

    echo "anchor: $anchor"
    echo "rules:"
    pfctl -a "$anchor" -sr 2>/dev/null || echo "  (none)"
}

# Main dispatch
if [ $# -lt 1 ]; then
    usage
fi

case "$1" in
    activate)
        shift
        cmd_activate "$@"
        ;;
    deactivate)
        shift
        cmd_deactivate "$@"
        ;;
    status)
        shift
        cmd_status "$@"
        ;;
    *)
        usage
        ;;
esac
