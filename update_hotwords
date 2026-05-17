#!/usr/bin/env python3
"""
Fetch Apple Music top-50 charts and update the auto-generated section of
hotwords.txt with trending artist and song names.

Charts fetched: US, Greece, Germany, UK (proxy for global — Apple has no
single global chart in the iTunes RSS format).

Run manually:
    python3 update_hotwords.py

Or via systemd timer (see homeassistant-hotwords.timer).
The listener reloads hotwords automatically when this file changes.
"""

import json
import os
import sys
import urllib.request
from datetime import date

CHARTS = {
    "US":      "https://itunes.apple.com/us/rss/topsongs/limit=50/json",
    "Greece":  "https://itunes.apple.com/gr/rss/topsongs/limit=50/json",
    "Germany": "https://itunes.apple.com/de/rss/topsongs/limit=50/json",
    "Global":  "https://itunes.apple.com/gb/rss/topsongs/limit=50/json",
}

HOTWORDS_FILE     = os.path.join(os.path.dirname(os.path.abspath(__file__)), "hotwords.txt")
AUTO_MARKER_START = "# --- auto: chart hotwords"
AUTO_MARKER_END   = "# --- end auto ---"


def fetch_chart(name, url):
    try:
        with urllib.request.urlopen(url, timeout=10) as r:
            data = json.loads(r.read())
        entries = data.get("feed", {}).get("entry", [])
        words = set()
        for e in entries:
            artist = e.get("im:artist", {}).get("label", "").strip()
            song   = e.get("im:name",   {}).get("label", "").strip()
            if artist:
                words.add(artist)
            if song:
                words.add(song)
        print(f"  {name}: {len(entries)} entries → {len(words)} terms")
        return words
    except Exception as e:
        print(f"  {name}: failed ({e})", file=sys.stderr)
        return set()


def read_manual_section():
    """Return lines before the auto block, preserving user edits."""
    try:
        with open(HOTWORDS_FILE) as f:
            lines = f.readlines()
    except FileNotFoundError:
        return []

    manual = []
    for line in lines:
        if line.startswith(AUTO_MARKER_START):
            break
        manual.append(line)

    while manual and manual[-1].strip() == "":
        manual.pop()
    return manual


def write_hotwords(all_words):
    manual = read_manual_section()
    today  = date.today().isoformat()

    out = manual + ["\n"]
    out.append(f"{AUTO_MARKER_START} — updated {today} ---\n")
    for w in sorted(all_words):
        out.append(f"{w}\n")
    out.append(f"{AUTO_MARKER_END}\n")

    with open(HOTWORDS_FILE, "w") as f:
        f.writelines(out)

    print(f"Wrote {len(all_words)} terms to {HOTWORDS_FILE}")


def main():
    print("Fetching charts...")
    all_words: set[str] = set()
    for name, url in CHARTS.items():
        all_words |= fetch_chart(name, url)

    if not all_words:
        print("No terms fetched — hotwords.txt not updated.", file=sys.stderr)
        sys.exit(1)

    write_hotwords(all_words)


if __name__ == "__main__":
    main()
