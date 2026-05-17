# Home Assistant Router

A minimal, headless home assistant for Arch Linux. Routes natural language input — typed or spoken — to a media subsystem (mpv + yt-dlp) or a persistent Claude-powered discussion harness.

## Requirements

```bash
sudo pacman -S mpv yt-dlp ffmpeg portaudio
```

## Installation

```bash
make build       # build binary
make install     # build + copy to ~/bin
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

Requires the Python listener service (Python 3.11 venv — openWakeWord is not compatible with Python 3.14+).

### One-time setup

```bash
make setup-listener                                 # creates .venv311 and installs deps
.venv311/bin/python listener --list-devices        # find your mic index or name
```

### Running

```bash
# Terminal 1 — router
./homeassistant --input-pipe /tmp/homeassistant.pipe

# Terminal 2 — voice listener
.venv311/bin/python listener --device <index or name> --pipe /tmp/homeassistant.pipe
```

Say **"Hey Jarvis"**, then speak your command. The transcript is routed automatically.

### Echo cancellation (recommended)

Without echo cancellation, the mic picks up audio played by mpv and can trigger false wake-word detections. PipeWire's WebRTC AEC module prevents this.

**1. Find your real output sink name:**

```bash
pactl list short sinks
# note the name, e.g. alsa_output.pci-0000_05_00.6.analog-stereo
```

**2. Create the PipeWire config:**

```bash
mkdir -p ~/.config/pipewire/pipewire.conf.d
```

`~/.config/pipewire/pipewire.conf.d/echo-cancel.conf` — replace `<your-sink>` with the name from step 1:

```
context.modules = [
  { name = libpipewire-module-echo-cancel
    args = {
      library.name = aec/libspa-aec-webrtc
      source.props   = { node.name = "Echo-Cancel Source" }
      sink.props     = { node.name = "Echo-Cancel Sink"   }
      playback.props = { node.target = "<your-sink>"      }
    }
  }
]
```

`playback.props.node.target` tells PipeWire where to forward audio from Echo-Cancel Sink so you still hear it through speakers.

**3. Restart PipeWire:**

```bash
systemctl --user restart pipewire pipewire-pulse
```

**4. Set Echo-Cancel Sink as the default output:**

AEC needs a reference signal — it only cancels audio that flows through Echo-Cancel Sink. Make it the default so mpv (and all other apps) route through it automatically:

```bash
pactl set-default-sink "Echo-Cancel Sink"
```

To persist this across reboots, add it to your autostart (e.g. `~/.config/autostart/` or a systemd user service that runs after `pipewire-pulse.service`).

**5. Verify:**

```bash
pactl info | grep "Default Sink"   # should say "Echo-Cancel Sink"
.venv311/bin/python listener --list-devices   # "Echo-Cancel Source" should appear
```

The systemd service (`homeassistant-listener.service`) is pre-configured to use `Echo-Cancel Source` automatically. With AEC active, the volume bar in the listener should stay near zero while music plays.

## Running as systemd user services

```bash
make enable    # install service files, daemon-reload, enable and start both services
```

Other service commands:

```bash
make start     # start both services
make stop      # stop both services
make restart   # restart both services
make disable   # stop and disable both services
make logs      # tail router logs
make logs-listener  # tail listener logs
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
make test
```
