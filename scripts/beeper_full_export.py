#!/usr/bin/env python3
"""
Beeper Full Export - iteratively paginate all chats, messages, and download media assets.
Uses the Beeper Desktop API via beeper-cli.
Token sourced from macOS Keychain (BEEPER_ACCESS_TOKEN).
"""

import json
import os
import subprocess
import sys
import time
import shutil
import sqlite3
import hashlib
from pathlib import Path
from datetime import datetime, timezone
from urllib.parse import urlparse, unquote

EXPORT_DIR = Path(os.environ.get("BEEPER_EXPORT_DIR", os.path.expanduser("~/i/beeper-export")))
BEEPER_CLI = os.path.expanduser("~/go/bin/beeper-cli")
BEEPER_DB = os.path.expanduser("~/Library/Application Support/BeeperTexts/index.db")
MEDIA_CACHE = os.path.expanduser("~/Library/Application Support/BeeperTexts/media")

def get_token():
    result = subprocess.run(
        ["security", "find-generic-password", "-a", "BEEPER_ACCESS_TOKEN", "-w"],
        capture_output=True, text=True
    )
    token = result.stdout.strip()
    if not token:
        print("ERROR: Could not retrieve BEEPER_ACCESS_TOKEN from keychain", file=sys.stderr)
        sys.exit(1)
    return token

def beeper_cli(args, token):
    cmd = [BEEPER_CLI] + args + ["-t", token, "-o", "json"]
    result = subprocess.run(cmd, capture_output=True, text=True, timeout=30)
    if result.returncode != 0:
        print(f"  CLI error: {result.stderr.strip()}", file=sys.stderr)
        return None
    try:
        return json.loads(result.stdout)
    except json.JSONDecodeError:
        print(f"  JSON parse error: {result.stdout[:200]}", file=sys.stderr)
        return None

def paginate_chats(token):
    """Iterate all chats with cursor-based pagination."""
    all_chats = []
    cursor = None
    page = 0
    while True:
        page += 1
        args = ["chats", "list"]
        if cursor:
            args += ["--cursor", cursor, "--direction", "before"]
        data = beeper_cli(args, token)
        if not data or "items" not in data:
            break
        items = data["items"]
        all_chats.extend(items)
        print(f"  Chats page {page}: {len(items)} chats (total: {len(all_chats)})")
        if not data.get("hasMore", False):
            break
        cursor = data.get("oldestCursor")
        if not cursor:
            break
        time.sleep(0.1)
    return all_chats

def paginate_messages(token, chat_id):
    """Iterate all messages in a chat with cursor-based pagination."""
    all_messages = []
    cursor = None
    page = 0
    while True:
        page += 1
        args = ["messages", "list", chat_id]
        if cursor:
            args += ["--cursor", cursor, "--direction", "before"]
        data = beeper_cli(args, token)
        if not data or "items" not in data:
            break
        items = data["items"]
        all_messages.extend(items)
        if not data.get("hasMore", False):
            break
        # Use the oldest sortKey as cursor
        if items:
            cursor = str(items[-1].get("sortKey", ""))
        if not cursor:
            break
        time.sleep(0.05)
    return all_messages

def download_asset(token, mxc_url, dest_path):
    """Download a media asset via beeper-cli or copy from local cache."""
    # Check local cache first
    if mxc_url.startswith("mxc://"):
        parsed = mxc_url.split("?")[0].replace("mxc://", "")
        cache_path = os.path.join(MEDIA_CACHE, parsed)
        if os.path.exists(cache_path):
            shutil.copy2(cache_path, dest_path)
            return True
    elif mxc_url.startswith("localmxc://"):
        parsed = mxc_url.replace("localmxc://", "")
        # Try several cache path patterns
        for prefix in ["", "localhost"]:
            cache_path = os.path.join(MEDIA_CACHE, prefix + parsed)
            if os.path.exists(cache_path):
                shutil.copy2(cache_path, dest_path)
                return True

    # Fall back to CLI download
    data = beeper_cli(["assets", "download", mxc_url.split("?")[0]], token)
    if data and "srcURL" in data:
        src = data["srcURL"]
        if src.startswith("file://"):
            local_path = unquote(src[7:])
            if os.path.exists(local_path):
                shutil.copy2(local_path, dest_path)
                return True
    return False

def get_extension(mime_type, filename=""):
    """Get file extension from mime type or filename."""
    if filename and "." in filename:
        ext = filename.rsplit(".", 1)[-1].lower()
        if len(ext) <= 5:
            return ext
    mime_map = {
        "image/jpeg": "jpg", "image/png": "png", "image/gif": "gif",
        "image/webp": "webp", "image/jp2": "jp2",
        "video/mp4": "mp4", "video/webm": "webm", "video/quicktime": "mov",
        "audio/aac": "aac", "audio/mpeg": "mp3", "audio/mp4": "m4a",
        "audio/wav": "wav", "audio/ogg": "ogg",
        "application/pdf": "pdf", "application/zip": "zip",
        "text/plain": "txt", "text/html": "html", "text/markdown": "md",
        "text/x-python": "py",
    }
    return mime_map.get(mime_type, "bin")

def export_from_sqlite():
    """Bulk export from the local SQLite database (fastest, most complete)."""
    print("\n=== Phase 1: Bulk export from SQLite database ===")
    
    conn = sqlite3.connect(f"file:{BEEPER_DB}?mode=ro", uri=True)
    conn.row_factory = sqlite3.Row

    # Export threads
    print("Exporting threads...")
    threads_dir = EXPORT_DIR / "threads"
    threads_dir.mkdir(parents=True, exist_ok=True)
    
    cursor = conn.execute("SELECT threadID, accountID, thread, timestamp FROM threads ORDER BY timestamp DESC")
    threads = []
    for row in cursor:
        thread_data = json.loads(row["thread"])
        thread_data["_threadID"] = row["threadID"]
        thread_data["_accountID"] = row["accountID"]
        thread_data["_timestamp"] = row["timestamp"]
        threads.append(thread_data)
    
    with open(threads_dir / "all_threads.jsonl", "w") as f:
        for t in threads:
            f.write(json.dumps(t, default=str) + "\n")
    print(f"  Exported {len(threads)} threads")

    # Export messages per room with media references
    print("Exporting messages...")
    messages_dir = EXPORT_DIR / "messages"
    messages_dir.mkdir(parents=True, exist_ok=True)
    
    media_manifest = []
    
    # Get all rooms
    rooms = conn.execute(
        "SELECT DISTINCT roomID FROM mx_room_messages ORDER BY roomID"
    ).fetchall()
    
    total_msgs = 0
    total_media = 0
    
    for room_row in rooms:
        room_id = room_row["roomID"]
        safe_room = room_id.replace("!", "").replace(":", "_").replace("/", "_")[:80]
        
        room_msgs = conn.execute("""
            SELECT eventID, roomID, senderContactID, timestamp, type, 
                   isSentByMe, isDeleted, isEncrypted, protocol, message,
                   text_content
            FROM mx_room_messages 
            WHERE roomID = ? AND isDeleted = 0
            ORDER BY timestamp ASC
        """, (room_id,)).fetchall()
        
        if not room_msgs:
            continue
            
        room_data = []
        for msg in room_msgs:
            msg_json = json.loads(msg["message"])
            record = {
                "eventID": msg["eventID"],
                "roomID": msg["roomID"],
                "sender": msg["senderContactID"],
                "timestamp": msg["timestamp"],
                "type": msg["type"],
                "isSentByMe": bool(msg["isSentByMe"]),
                "isEncrypted": bool(msg["isEncrypted"]),
                "protocol": msg["protocol"],
                "text": msg_json.get("text", msg["text_content"] or ""),
                "attachments": msg_json.get("attachments", []),
            }
            room_data.append(record)
            
            # Track media
            for att in msg_json.get("attachments", []):
                src_url = att.get("srcURL", att.get("id", ""))
                if src_url:
                    media_manifest.append({
                        "roomID": room_id,
                        "eventID": msg["eventID"],
                        "timestamp": msg["timestamp"],
                        "srcURL": src_url,
                        "mimeType": att.get("mimeType", ""),
                        "fileName": att.get("fileName", ""),
                        "fileSize": att.get("fileSize", 0),
                        "type": att.get("type", ""),
                    })
                    total_media += 1
        
        with open(messages_dir / f"{safe_room}.jsonl", "w") as f:
            for r in room_data:
                f.write(json.dumps(r, default=str) + "\n")
        
        total_msgs += len(room_data)
    
    print(f"  Exported {total_msgs} messages across {len(rooms)} rooms")
    print(f"  Found {total_media} media attachments")
    
    # Save media manifest
    media_dir = EXPORT_DIR / "media"
    media_dir.mkdir(parents=True, exist_ok=True)
    
    with open(EXPORT_DIR / "media_manifest.jsonl", "w") as f:
        for m in media_manifest:
            f.write(json.dumps(m) + "\n")
    
    # Export participants/user profiles
    print("Exporting user profiles...")
    users_dir = EXPORT_DIR / "users"
    users_dir.mkdir(parents=True, exist_ok=True)
    
    try:
        profiles = conn.execute("""
            SELECT user_id, room_id, displayname, avatar_url, membership
            FROM mx_user_profile
        """).fetchall()
        # This is in account.db, not index.db - handle gracefully
    except:
        profiles = []
    
    # Export participants from index.db
    try:
        participants = conn.execute("SELECT * FROM participants").fetchall()
        with open(users_dir / "participants.jsonl", "w") as f:
            for p in participants:
                f.write(json.dumps(dict(p), default=str) + "\n")
        print(f"  Exported {len(participants)} participants")
    except Exception as e:
        print(f"  Participants export: {e}")
    
    conn.close()
    return media_manifest

def download_media_assets(token, media_manifest):
    """Download all media assets, using cache where possible."""
    print(f"\n=== Phase 2: Downloading {len(media_manifest)} media assets ===")
    
    media_dir = EXPORT_DIR / "media"
    media_dir.mkdir(parents=True, exist_ok=True)
    
    downloaded = 0
    cached = 0
    failed = 0
    skipped = 0
    
    for i, item in enumerate(media_manifest):
        src_url = item["srcURL"]
        mime = item.get("mimeType", "")
        fname = item.get("fileName", "")
        ext = get_extension(mime, fname)
        
        # Create a safe filename from the URL hash
        url_hash = hashlib.sha256(src_url.encode()).hexdigest()[:16]
        safe_name = f"{url_hash}.{ext}"
        dest_path = media_dir / safe_name
        
        if dest_path.exists():
            skipped += 1
            continue
        
        success = download_asset(token, src_url, str(dest_path))
        if success:
            if "cache" in str(dest_path):
                cached += 1
            else:
                downloaded += 1
        else:
            failed += 1
        
        if (i + 1) % 100 == 0:
            print(f"  Progress: {i+1}/{len(media_manifest)} (downloaded: {downloaded}, cached: {cached}, failed: {failed}, skipped: {skipped})")
    
    print(f"  Final: downloaded={downloaded}, cached/copied={cached}, failed={failed}, skipped={skipped}")
    
    # Update manifest with local paths
    with open(EXPORT_DIR / "media_manifest_with_paths.jsonl", "w") as f:
        for item in media_manifest:
            url_hash = hashlib.sha256(item["srcURL"].encode()).hexdigest()[:16]
            ext = get_extension(item.get("mimeType", ""), item.get("fileName", ""))
            local_name = f"{url_hash}.{ext}"
            local_path = media_dir / local_name
            item["localPath"] = str(local_path) if local_path.exists() else None
            f.write(json.dumps(item) + "\n")

def enrich_with_api(token):
    """Use the live API to get additional data not in SQLite."""
    print("\n=== Phase 3: Enriching with live API data ===")
    
    api_dir = EXPORT_DIR / "api"
    api_dir.mkdir(parents=True, exist_ok=True)
    
    # Get all chats with full metadata via pagination
    print("Paginating all chats from API...")
    all_chats = paginate_chats(token)
    
    with open(api_dir / "all_chats.jsonl", "w") as f:
        for c in all_chats:
            f.write(json.dumps(c, default=str) + "\n")
    print(f"  Exported {len(all_chats)} chats from API")
    
    # Get accounts info
    accounts = beeper_cli(["accounts", "list"], token)
    if accounts:
        with open(api_dir / "accounts.json", "w") as f:
            json.dump(accounts, f, indent=2, default=str)
        print(f"  Exported accounts info")

def generate_summary(media_manifest):
    """Generate a summary report of the export."""
    print("\n=== Generating export summary ===")
    
    summary = {
        "exportDate": datetime.now(timezone.utc).isoformat(),
        "exportDir": str(EXPORT_DIR),
        "totalMediaAssets": len(media_manifest),
        "mediaByType": {},
        "mediaByMime": {},
        "totalMediaSizeBytes": 0,
    }
    
    for item in media_manifest:
        t = item.get("type", "unknown")
        m = item.get("mimeType", "unknown")
        summary["mediaByType"][t] = summary["mediaByType"].get(t, 0) + 1
        summary["mediaByMime"][m] = summary["mediaByMime"].get(m, 0) + 1
        summary["totalMediaSizeBytes"] += item.get("fileSize", 0)
    
    # Count messages
    msgs_dir = EXPORT_DIR / "messages"
    total_msgs = 0
    total_rooms = 0
    if msgs_dir.exists():
        for f in msgs_dir.glob("*.jsonl"):
            total_rooms += 1
            with open(f) as fh:
                total_msgs += sum(1 for _ in fh)
    
    summary["totalMessages"] = total_msgs
    summary["totalRooms"] = total_rooms
    
    # Count downloaded media
    media_dir = EXPORT_DIR / "media"
    if media_dir.exists():
        media_files = list(media_dir.glob("*"))
        summary["downloadedMediaFiles"] = len(media_files)
        summary["downloadedMediaSizeBytes"] = sum(f.stat().st_size for f in media_files if f.is_file())
    
    with open(EXPORT_DIR / "export_summary.json", "w") as f:
        json.dump(summary, f, indent=2)
    
    print(f"\nExport Summary:")
    print(f"  Messages: {total_msgs}")
    print(f"  Rooms: {total_rooms}")
    print(f"  Media assets: {len(media_manifest)}")
    print(f"  Media size: {summary['totalMediaSizeBytes'] / 1024 / 1024:.1f} MB (referenced)")
    print(f"  Downloaded: {summary.get('downloadedMediaFiles', 0)} files")
    print(f"  Export dir: {EXPORT_DIR}")

def main():
    print(f"Beeper Full Export - {datetime.now().isoformat()}")
    print(f"Export directory: {EXPORT_DIR}")
    EXPORT_DIR.mkdir(parents=True, exist_ok=True)
    
    token = get_token()
    print(f"Token acquired (len={len(token)})")
    
    # Phase 1: Bulk export from SQLite
    media_manifest = export_from_sqlite()
    
    # Phase 2: Download media assets
    download_media_assets(token, media_manifest)
    
    # Phase 3: Enrich with API
    enrich_with_api(token)
    
    # Summary
    generate_summary(media_manifest)
    
    print(f"\nExport complete! See {EXPORT_DIR}")

if __name__ == "__main__":
    main()
