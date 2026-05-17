# Home Assistant Router

A minimal, headless home assistant for Arch Linux. Routes natural language input — typed or spoken — to a media subsystem (mpv + yt-dlp) or a persistent Claude-powered discussion harness.

## Requirements

```bash
sudo pacman -S mpv yt-dlp ffmpeg portaudio
```

## Installation

```bash
go build -o homeassistant .
```

## Environment variables

Create `~/.config/homeassistant/env`:

```
HOMEASSISTANT_LLM_URL=https://api.anthropic.com/v1/messages
HOMEASSISTANT_API_KEY=sk-ant-...
HOMEASSISTANT_MODEL=claude-sonnet-4-6
```

For local development, direnv is supported — copy to `.envrc` with `export` prefixes:

```bash
cp ~/.config/homeassistant/env .envrc
# prepend `export ` to each line, then:
direnv allow
```

## Text mode

```bash
./homeassistant
```

Type commands directly:

| Input | Action |
|---|---|
| `play <query>` / `search <query>` | Search YouTube and stream audio |
| `louder` / `quieter` / `volume up` / `volume down` | Adjust volume ±10% |
| `pause` / `resume` / `continue` / `unpause` | Pause or resume playback |
| `stop` / `stop playing` / `stop music` | Stop playback |
| anything else | Routed to Claude for conversation |

Natural language is also supported for media commands (e.g. "put on some jazz", "turn it down a bit") via the LLM classifier fallback.

## Voice mode

Requires the Python listener service.

### One-time setup

```bash
./setup-listener.sh                           # creates .venv and installs deps
.venv/bin/python listener.py --list-devices   # find your USB mic index
```

### Running

```bash
# Terminal 1 — router
./homeassistant --input-pipe /tmp/homeassistant.pipe

# Terminal 2 — voice listener
.venv/bin/python listener.py --device <index> --pipe /tmp/homeassistant.pipe
```

Say **"Hey Jarvis"**, then speak your command. The transcript is routed automatically.

## Running as systemd user services

```bash
cp homeassistant-router.service homeassistant-listener.service ~/.config/systemd/user/
systemctl --user daemon-reload
systemctl --user enable --now homeassistant-router homeassistant-listener
```

View logs:

```bash
journalctl --user -f -u homeassistant-router
journalctl --user -f -u homeassistant-listener
```

## Routing pipeline

1. **Fast path** — exact match against `~/.config/homeassistant/routes.jsonl` (auto-created from defaults on first run, manually editable — append lines to add new commands)
2. **Prefix match** — hardcoded `play` / `search` with payload extraction
3. **LLM classifier** — stateless Claude API call returning intent + payload
4. **Discussion fallback** — full conversation history sent to Claude

## Conversation state

Conversation history is persisted to `~/.config/homeassistant/state.json` and evicted automatically at the start of each new day.

## Running tests

```bash
go test ./...
```
