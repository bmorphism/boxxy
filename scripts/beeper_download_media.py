#!/usr/bin/env python3
"""
Fast media download - copies from Beeper's local cache, falls back to API.
Designed to be re-run safely (skips existing files).
"""

import json
import os
import shutil
import subprocess
import hashlib
import sys
from pathlib import Path
from concurrent.futures import ThreadPoolExecutor, as_completed

EXPORT_DIR = Path(os.path.expanduser("~/i/beeper-export"))
MEDIA_DIR = EXPORT_DIR / "media"
MANIFEST = EXPORT_DIR / "media_manifest.jsonl"
MEDIA_CACHE = Path(os.path.expanduser("~/Library/Application Support/BeeperTexts/media"))
BEEPER_CLI = os.path.expanduser("~/go/bin/beeper-cli")

def get_token():
    result = subprocess.run(
        ["security", "find-generic-password", "-a", "BEEPER_ACCESS_TOKEN", "-w"],
        capture_output=True, text=True
    )
    return result.stdout.strip()

def get_extension(mime_type, filename=""):
    if filename and "." in filename:
        ext = filename.rsplit(".", 1)[-1].lower()
        if len(ext) <= 5 and ext.isalnum():
            return ext
    mime_map = {
        "image/jpeg": "jpg", "image/png": "png", "image/gif": "gif",
        "image/webp": "webp", "image/jp2": "jp2",
        "video/mp4": "mp4", "video/webm": "webm", "video/quicktime": "mov",
        "audio/aac": "aac", "audio/mpeg": "mp3", "audio/mp4": "m4a",
        "audio/wav": "wav", "audio/ogg": "ogg",
        "application/pdf": "pdf", "application/zip": "zip",
        "text/plain": "txt", "text/html": "html", "text/markdown": "md",
        "text/x-python": "py", "application/octet-stream": "bin",
    }
    return mime_map.get(mime_type, "bin")

def find_in_cache(src_url):
    """Try to find the file in Beeper's local media cache."""
    base_url = src_url.split("?")[0]

    if base_url.startswith("mxc://"):
        rel = base_url[6:]  # strip mxc://
        cache_path = MEDIA_CACHE / rel
        if cache_path.exists():
            return cache_path

    elif base_url.startswith("localmxc://"):
        rel = base_url[11:]  # strip localmxc://
        # Try multiple patterns
        for prefix in ["", "localhost"]:
            cache_path = MEDIA_CACHE / (prefix + rel)
            if cache_path.exists():
                return cache_path
        # Also try with the server part as directory
        parts = rel.split("/", 1)
        if len(parts) == 2:
            for suffix in [".localhost", ""]:
                cache_path = MEDIA_CACHE / (parts[0] + suffix) / parts[1]
                if cache_path.exists():
                    return cache_path

    # Brute force: search all subdirectories of media cache
    # Extract the unique ID part from the URL
    url_id = base_url.rsplit("/", 1)[-1] if "/" in base_url else base_url
    for subdir in MEDIA_CACHE.iterdir():
        if subdir.is_dir():
            candidate = subdir / url_id
            if candidate.exists():
                return candidate

    return None

def download_one(item, token):
    """Download a single media item. Returns (status, item)."""
    src_url = item["srcURL"]
    mime = item.get("mimeType", "")
    fname = item.get("fileName", "")
    ext = get_extension(mime, fname)
    url_hash = hashlib.sha256(src_url.encode()).hexdigest()[:16]
    safe_name = f"{url_hash}.{ext}"
    dest_path = MEDIA_DIR / safe_name

    if dest_path.exists() and dest_path.stat().st_size > 0:
        return ("skipped", item, str(dest_path))

    # Try cache first
    cache_path = find_in_cache(src_url)
    if cache_path:
        shutil.copy2(cache_path, dest_path)
        return ("cached", item, str(dest_path))

    # Fall back to beeper-cli
    base_url = src_url.split("?")[0]
    try:
        result = subprocess.run(
            [BEEPER_CLI, "assets", "download", base_url, "-t", token, "-o", "json"],
            capture_output=True, text=True, timeout=30
        )
        if result.returncode == 0:
            data = json.loads(result.stdout)
            if "srcURL" in data and data["srcURL"].startswith("file://"):
                from urllib.parse import unquote
                local_path = unquote(data["srcURL"][7:])
                if os.path.exists(local_path):
                    shutil.copy2(local_path, dest_path)
                    return ("downloaded", item, str(dest_path))
    except Exception:
        pass

    return ("failed", item, None)

def main():
    MEDIA_DIR.mkdir(parents=True, exist_ok=True)
    token = get_token()

    items = []
    with open(MANIFEST) as f:
        for line in f:
            items.append(json.loads(line))

    print(f"Processing {len(items)} media items...")

    stats = {"skipped": 0, "cached": 0, "downloaded": 0, "failed": 0}
    failed_items = []

    # Use thread pool for parallel CLI calls (serial for cache copies)
    # First pass: try cache (fast, single-threaded)
    api_needed = []
    for item in items:
        src_url = item["srcURL"]
        mime = item.get("mimeType", "")
        fname = item.get("fileName", "")
        ext = get_extension(mime, fname)
        url_hash = hashlib.sha256(src_url.encode()).hexdigest()[:16]
        dest_path = MEDIA_DIR / f"{url_hash}.{ext}"

        if dest_path.exists() and dest_path.stat().st_size > 0:
            stats["skipped"] += 1
            continue

        cache_path = find_in_cache(src_url)
        if cache_path:
            shutil.copy2(cache_path, dest_path)
            stats["cached"] += 1
        else:
            api_needed.append(item)

    print(f"Cache pass: skipped={stats['skipped']}, cached={stats['cached']}, need API={len(api_needed)}")

    if api_needed:
        print(f"Downloading {len(api_needed)} items via API (parallel)...")
        with ThreadPoolExecutor(max_workers=4) as executor:
            futures = {executor.submit(download_one, item, token): item for item in api_needed}
            done = 0
            for future in as_completed(futures):
                status, item, path = future.result()
                stats[status] = stats.get(status, 0) + 1
                if status == "failed":
                    failed_items.append(item)
                done += 1
                if done % 50 == 0:
                    print(f"  API progress: {done}/{len(api_needed)}")

    print(f"\nFinal stats:")
    print(f"  Skipped (already exist): {stats['skipped']}")
    print(f"  Copied from cache: {stats['cached']}")
    print(f"  Downloaded via API: {stats.get('downloaded', 0)}")
    print(f"  Failed: {stats.get('failed', 0)}")
    print(f"  Total media files: {len(list(MEDIA_DIR.glob('*')))}")

    # Save updated manifest with local paths
    with open(EXPORT_DIR / "media_manifest_with_paths.jsonl", "w") as f:
        for item in items:
            url_hash = hashlib.sha256(item["srcURL"].encode()).hexdigest()[:16]
            ext = get_extension(item.get("mimeType", ""), item.get("fileName", ""))
            local_path = MEDIA_DIR / f"{url_hash}.{ext}"
            item["localPath"] = str(local_path) if local_path.exists() else None
            f.write(json.dumps(item) + "\n")

    if failed_items:
        with open(EXPORT_DIR / "media_failed.jsonl", "w") as f:
            for item in failed_items:
                f.write(json.dumps(item) + "\n")
        print(f"  Failed items saved to media_failed.jsonl")

    # Total size
    total_size = sum(f.stat().st_size for f in MEDIA_DIR.glob("*") if f.is_file())
    print(f"  Total media size: {total_size / 1024 / 1024:.1f} MB")

if __name__ == "__main__":
    main()
