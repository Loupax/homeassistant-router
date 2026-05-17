#!/usr/bin/env python3
"""
Voice listener for Home Assistant Router.

Dependencies (pip):
    sounddevice
    numpy
    faster-whisper
    torch torchaudio   (for silero-vad)
    scipy

System packages (pacman):
    portaudio          (already installed)
    ffmpeg

Usage:
    python listener.py [--device INDEX] [--pipe /tmp/homeassistant.pipe]

Device index: run `python listener.py --list-devices` to see available inputs.

# Install dependencies:
#   pip install sounddevice numpy faster-whisper torch torchaudio scipy
#   sudo pacman -S portaudio ffmpeg
"""

import argparse
import json
import socket
import sys
import time

import numpy as np
import sounddevice as sd
import torch
from scipy.signal import resample_poly
from faster_whisper import WhisperModel

SAMPLE_RATE = 16000        # Hz — required by all models
CHUNK_MS    = 32           # ms per chunk — Silero VAD requires exactly 512 samples @ 16kHz
CHUNK_SIZE  = int(SAMPLE_RATE * CHUNK_MS / 1000)  # 512 samples

MAX_WAKE_SECONDS    = 3    # max recording length for wake phrase detection
MAX_COMMAND_SECONDS = 10   # max recording length for command

SILENCE_WAKE_SECONDS    = 0.7  # silence to end wake phrase recording
SILENCE_COMMAND_SECONDS = 0.7  # silence to end command recording

PIPE_PATH = "/tmp/homeassistant.pipe"
WAKE_PHRASE = "hey jarvis"
VAD_ONSET_THRESHOLD = 0.5
MPV_SOCKET = "/tmp/ha-mpv.sock"
DUCK_VOLUME = 5

# ---------------------------------------------------------------------------
# Model initialisation (module level — loaded once at startup)
# ---------------------------------------------------------------------------

vad_model, _ = torch.hub.load(
    'snakers4/silero-vad', 'silero_vad', force_reload=False, trust_repo=True
)

whisper_wake = WhisperModel("Systran/faster-distil-whisper-small.en", device="cpu", compute_type="int8")
whisper_cmd  = whisper_wake  # same model; phase 1 audio is short so it's still fast


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def resample_to_model(chunk, device_rate):
    """Convert int16 chunk from device_rate to SAMPLE_RATE float32 [-1, 1]."""
    pcm = chunk.flatten().astype('float32') / 32768.0
    if device_rate == SAMPLE_RATE:
        return pcm
    return resample_poly(pcm, SAMPLE_RATE, device_rate).astype('float32')


def open_pipe_with_retry(path):
    """Open the FIFO for writing, retrying until the reader (router) is up."""
    while True:
        try:
            fd = open(path, 'w')
            return fd
        except (FileNotFoundError, OSError) as e:
            print(f"Pipe not ready ({e}), retrying in 2s...")
            time.sleep(2)


def vad_score(chunk_np):
    """Return Silero VAD speech probability for a float32 16kHz chunk."""
    tensor = torch.tensor(chunk_np)
    return vad_model(tensor, SAMPLE_RATE).item()


def record_until_silence(stream, device_rate, silence_seconds, max_seconds, initial_chunk=None):
    """Record audio chunks until Silero VAD detects sustained silence."""
    buffer = [initial_chunk] if initial_chunk is not None else []
    silence_chunks = 0
    device_chunk  = int(device_rate * CHUNK_MS / 1000)
    max_chunks    = int(max_seconds * 1000 / CHUNK_MS)
    silence_limit = int(silence_seconds * 1000 / CHUNK_MS)

    for _ in range(max_chunks):
        chunk, _ = stream.read(device_chunk)
        chunk_np = resample_to_model(chunk, device_rate)
        buffer.append(chunk_np)

        if vad_score(chunk_np) < 0.5:
            silence_chunks += 1
            if silence_chunks >= silence_limit:
                break
        else:
            silence_chunks = 0

    return np.concatenate(buffer)


_COMMAND_PROMPT = "Hey Jarvis, play music, stop, louder, quieter, pause, resume, what's the weather."

def transcribe_wake(audio_np):
    """Fast wake-phrase detection with tiny.en, greedy decoding."""
    segments, _ = whisper_wake.transcribe(
        audio_np, language="en", beam_size=1, vad_filter=True,
    )
    return " ".join(s.text.strip() for s in segments).strip()


def transcribe_command(audio_np):
    """Accurate command transcription with small.en, greedy decoding."""
    segments, _ = whisper_cmd.transcribe(
        audio_np,
        language="en",
        beam_size=1,
        initial_prompt=_COMMAND_PROMPT,
        vad_filter=True,
    )
    return " ".join(s.text.strip() for s in segments).strip()


def strip_wake_phrase(text):
    """Find wake phrase anywhere in text (case-insensitive). Returns everything after it, or None."""
    lower = text.lower().strip()
    idx = lower.find(WAKE_PHRASE)
    if idx == -1:
        return None
    return text[idx + len(WAKE_PHRASE):].strip(" ,.")


def _mpv_command(command):
    """Send a JSON command to the mpv IPC socket; returns response data or None."""
    try:
        with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as s:
            s.settimeout(0.5)
            s.connect(MPV_SOCKET)
            s.sendall((json.dumps({"command": command}) + "\n").encode())
            data = json.loads(s.recv(4096).decode().strip())
            return data.get("data")
    except (FileNotFoundError, ConnectionRefusedError, OSError, json.JSONDecodeError):
        return None


def duck_volume():
    """Lower mpv volume to DUCK_VOLUME%; return previous volume (or None)."""
    prev = _mpv_command(["get_property", "volume"])
    if prev is not None:
        _mpv_command(["set_property", "volume", DUCK_VOLUME])
    return prev


def unduck_volume(prev):
    """Restore mpv volume to the value returned by duck_volume()."""
    if prev is not None:
        _mpv_command(["set_property", "volume", prev])


_GRAY   = "\033[90m"
_YELLOW = "\033[33m"
_GREEN  = "\033[32m"
_CYAN   = "\033[36m"
_RED    = "\033[31m"
_RESET  = "\033[0m"

def status(msg):
    """Print a full-width status line, clearing the VU meter."""
    print(f"\r\033[K{msg}", flush=True)


def write_to_pipe(pipe_fd, text, pipe_path):
    """Write a line to the FIFO, reconnecting if the router has gone away."""
    try:
        pipe_fd.write(text + "\n")
        pipe_fd.flush()
    except BrokenPipeError:
        print("Router disconnected. Waiting to reconnect...")
        pipe_fd = open_pipe_with_retry(pipe_path)
    return pipe_fd


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

def main():
    parser = argparse.ArgumentParser(
        description="Voice listener for Home Assistant Router"
    )
    parser.add_argument(
        "--device",
        type=int,
        default=None,
        help="sounddevice input device index (default: system default)",
    )
    parser.add_argument(
        "--pipe",
        default=PIPE_PATH,
        help=f"FIFO path (default: {PIPE_PATH})",
    )
    parser.add_argument(
        "--list-devices",
        action="store_true",
        help="print available input devices and exit",
    )
    args = parser.parse_args()

    if args.list_devices:
        print("Available input devices:")
        for i, dev in enumerate(sd.query_devices()):
            if dev['max_input_channels'] > 0:
                print(f"  [{i}] {dev['name']}")
        sys.exit(0)

    pipe_fd = open_pipe_with_retry(args.pipe)

    device_info  = sd.query_devices(args.device, 'input')
    device_rate  = int(device_info['default_samplerate'])
    device_chunk = int(device_rate * CHUNK_MS / 1000)
    print(f"Device sample rate: {device_rate}Hz (resampling to {SAMPLE_RATE}Hz)")
    print(f"Wake phrase: \"{WAKE_PHRASE}\"")

    with sd.InputStream(
        samplerate=device_rate,
        channels=1,
        dtype='int16',
        blocksize=device_chunk,
        device=args.device,
    ) as stream:
        print(f"{_GRAY}Listening for \"{WAKE_PHRASE}\"... (Ctrl+C to quit){_RESET}")
        while True:
            chunk, _ = stream.read(device_chunk)
            chunk_np = resample_to_model(chunk, device_rate)

            rms      = float(np.sqrt(np.mean(chunk_np ** 2)))
            vol_bars = int(min(rms * 400, 30))
            prob     = vad_score(chunk_np)
            vad_bars = int(prob * 10)

            print(
                f"\r{_GRAY}🎙  [{'█' * vol_bars:<30}]  vad:{prob:.2f} [{'█' * vad_bars:<10}]{_RESET}",
                end="", flush=True,
            )

            if prob < VAD_ONSET_THRESHOLD:
                continue

            # ---- Phase 1: detect wake phrase ----
            t0 = time.monotonic()
            status(f"{_YELLOW}⏺  Recording...{_RESET}")
            wake_audio = record_until_silence(
                stream, device_rate,
                silence_seconds=SILENCE_WAKE_SECONDS,
                max_seconds=MAX_WAKE_SECONDS,
                initial_chunk=chunk_np,
            )
            t_rec = time.monotonic()
            status(f"{_YELLOW}⏳  Transcribing...  (recorded {t_rec - t0:.1f}s){_RESET}")
            wake_transcript = transcribe_wake(wake_audio)
            t_tr = time.monotonic()
            if not wake_transcript:
                status(f"{_GRAY}Listening for \"{WAKE_PHRASE}\"... (Ctrl+C to quit){_RESET}")
                continue

            inline_command = strip_wake_phrase(wake_transcript)
            if inline_command is None:
                status(f"{_GRAY}Ignored: {wake_transcript}  (transcribe {t_tr - t_rec:.1f}s){_RESET}")
                status(f"{_GRAY}Listening for \"{WAKE_PHRASE}\"... (Ctrl+C to quit){_RESET}")
                continue

            # ---- Phase 2: record command ----
            prev_vol = duck_volume()
            try:
                if inline_command:
                    command = inline_command
                else:
                    status(f"{_GREEN}✓  Wake phrase!  (transcribe {t_tr - t_rec:.1f}s)  Listening for command...{_RESET}")
                    t1 = time.monotonic()
                    cmd_audio = record_until_silence(
                        stream, device_rate,
                        silence_seconds=SILENCE_COMMAND_SECONDS,
                        max_seconds=MAX_COMMAND_SECONDS,
                    )
                    t_rec2 = time.monotonic()
                    status(f"{_YELLOW}⏳  Transcribing command...  (recorded {t_rec2 - t1:.1f}s){_RESET}")
                    command = transcribe_command(cmd_audio)
                    t_tr2 = time.monotonic()
                    status(f"{_YELLOW}⏳  Done  (transcribe {t_tr2 - t_rec2:.1f}s){_RESET}")

                if command:
                    status(f"{_CYAN}▶  {command}{_RESET}")
                    pipe_fd = write_to_pipe(pipe_fd, command, args.pipe)
                else:
                    status(f"{_RED}✗  No command heard{_RESET}")
            finally:
                unduck_volume(prev_vol)

            status(f"{_GRAY}Listening for \"{WAKE_PHRASE}\"... (Ctrl+C to quit){_RESET}")


if __name__ == "__main__":
    main()
