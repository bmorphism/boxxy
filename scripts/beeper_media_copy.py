#!/usr/bin/env python3
"""
Fast media copy from Beeper's local cache.
Reads media_manifest.jsonl, copies files from ~/Library/Application Support/BeeperTexts/media/.
Falls back to beeper-cli for missing files. Idempotent (skips existing).
"""
import json
import os
import shutil
import hashlib
import subprocess
import sys
from pathlib import Path
from concurrent.futures import ThreadPoolExecutor, as_completed
from urllib.parse import unquote

EXPORT_DIR = Path(os.path.expanduser("~/i/beeper-export"))
MEDIA_DIR = EXPORT_DIR / "media"
MANIFEST = EXPORT_DIR / "media_manifest.jsonl"
MEDIA_CACHE = Path(os.path.expanduser("~/Library/Application Support/BeeperTexts/media"))
BEEPER_CLI = os.path.expanduser("~/go/bin/beeper-cli")

MIME_MAP = {
    "image/jpeg": "jpg", "image/png": "png", "image/gif": "gif",
    "image/webp": "webp", "image/jp2": "jp2", "image/heic": "heic",
    "video/mp4": "mp4", "video/webm": "webm", "video/quicktime": "mov",
    "audio/aac": "aac", "audio/mpeg": "mp3", "audio/mp4": "m4a",
    "audio/wav": "wav", "audio/ogg": "ogg",
    "application/pdf": "pdf", "application/zip": "zip",
    "text/plain": "txt", "text/html": "html",
}

def get_ext(mime, filename=""):
    if filename and "." in filename:
        ext = filename.rsplit(".", 1)[-1].lower()
        if len(ext) <= 5:
            return ext
    return MIME_MAP.get(mime, "bin")

def find_in_cache(src_url):
    """Try to find the media file in Beeper's local cache."""
    base_url = src_url.split("?")[0]

    if base_url.startswith("mxc://"):
        rel = base_url.replace("mxc://", "")
        cache_path = MEDIA_CACHE / rel
        if cache_path.exists():
            return cache_path
    elif base_url.startswith("localmxc://"):
        rel = base_url.replace("localmxc://", "")
        # Try direct and with localhost prefix
        for prefix in ["", "localhost/"]:
            cache_path = MEDIA_CACHE / (prefix + rel)
            if cache_path.exists():
                return cache_path

    # Try searching by the last path component
    parts = base_url.replace("mxc://", "").replace("localmxc://", "").split("/")
    if len(parts) >= 2:
        server = parts[0]
        media_id = parts[-1]
        # Walk the cache looking for matching server/id
        server_dir = MEDIA_CACHE / server
        if server_dir.exists():
            candidate = server_dir / media_id
            if candidate.exists():
                return candidate

    return None

def copy_one(item, token=None):
    """Copy a single media item. Returns (status, item)."""
    src_url = item["srcURL"]
    mime = item.get("mimeType", "")
    fname = item.get("fileName", "")
    ext = get_ext(mime, fname)

    url_hash = hashlib.sha256(src_url.encode()).hexdigest()[:16]
    safe_name = f"{url_hash}.{ext}"
    dest = MEDIA_DIR / safe_name

    if dest.exists():
        return "skip", item

    # Try local cache
    cache_path = find_in_cache(src_url)
    if cache_path:
        shutil.copy2(str(cache_path), str(dest))
        return "cache", item

    # Try beeper-cli
    if token and os.path.exists(BEEPER_CLI):
        try:
            base_url = src_url.split("?")[0]
            result = subprocess.run(
                [BEEPER_CLI, "assets", "download", base_url, "-t", token, "-o", "json"],
                capture_output=True, text=True, timeout=30
            )
            if result.returncode == 0:
                data = json.loads(result.stdout)
                if "srcURL" in data:
                    local = data["srcURL"]
                    if local.startswith("file://"):
                        local_path = unquote(local[7:])
                        if os.path.exists(local_path):
                            shutil.copy2(local_path, str(dest))
                            return "api", item
        except Exception:
            pass

    return "fail", item

def get_token():
    try:
        result = subprocess.run(
            ["security", "find-generic-password", "-a", "BEEPER_ACCESS_TOKEN", "-w"],
            capture_output=True, text=True
        )
        return result.stdout.strip() or None
    except Exception:
        return None

def main():
    if not MANIFEST.exists():
        print(f"No manifest at {MANIFEST}")
        sys.exit(1)

    MEDIA_DIR.mkdir(parents=True, exist_ok=True)

    # Load manifest
    items = []
    with open(MANIFEST) as f:
        for line in f:
            line = line.strip()
            if line:
                items.append(json.loads(line))

    print(f"Media manifest: {len(items)} items")
    print(f"Cache dir: {MEDIA_CACHE} ({sum(f.stat().st_size for f in MEDIA_CACHE.rglob('*') if f.is_file()) / 1e6:.0f} MB)")

    token = get_token()
    if token:
        print(f"API token: available (len={len(token)})")
    else:
        print("API token: not found, cache-only mode")

    stats = {"skip": 0, "cache": 0, "api": 0, "fail": 0}

    # Phase 1: fast cache copy (parallel)
    print("\nPhase 1: copying from local cache (8 threads)...")
    with ThreadPoolExecutor(max_workers=8) as pool:
        futures = {pool.submit(copy_one, item): item for item in items}
        done = 0
        for future in as_completed(futures):
            status, item = future.result()
            stats[status] += 1
            done += 1
            if done % 500 == 0:
                print(f"  {done}/{len(items)} -- skip={stats['skip']} cache={stats['cache']} fail={stats['fail']}")

    # Phase 2: retry failures with API (sequential, slower)
    failures = stats["fail"]
    if failures > 0 and token:
        print(f"\nPhase 2: retrying {failures} failures via API...")
        retry_items = []
        with open(MANIFEST) as f:
            for line in f:
                item = json.loads(line.strip())
                url_hash = hashlib.sha256(item["srcURL"].encode()).hexdigest()[:16]
                ext = get_ext(item.get("mimeType", ""), item.get("fileName", ""))
                dest = MEDIA_DIR / f"{url_hash}.{ext}"
                if not dest.exists():
                    retry_items.append(item)

        for i, item in enumerate(retry_items):
            status, _ = copy_one(item, token=token)
            if status != "fail":
                stats["fail"] -= 1
                stats[status] += 1
            if (i + 1) % 100 == 0:
                print(f"  Retry {i+1}/{len(retry_items)}")

    # Summary
    total_files = len(list(MEDIA_DIR.glob("*")))
    total_size = sum(f.stat().st_size for f in MEDIA_DIR.glob("*") if f.is_file())

    print(f"\nMedia copy complete:")
    print(f"  Skipped (existing): {stats['skip']}")
    print(f"  From cache: {stats['cache']}")
    print(f"  From API: {stats['api']}")
    print(f"  Failed: {stats['fail']}")
    print(f"  Total files: {total_files}")
    print(f"  Total size: {total_size / 1e6:.1f} MB")

if __name__ == "__main__":
    main()
