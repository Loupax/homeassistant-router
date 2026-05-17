#!/usr/bin/env python3
"""
Voice listener for Home Assistant Router.

Requires Python 3.11 venv (.venv311) — openWakeWord TFLite backend is
not compatible with Python 3.14.

Dependencies (.venv311):
    pip install sounddevice numpy "numpy<2" faster-whisper torch torchaudio scipy openwakeword

System packages (pacman):
    portaudio ffmpeg

Usage:
    .venv311/bin/python listener.py [--device INDEX] [--pipe /tmp/homeassistant.pipe]

First run downloads openWakeWord models (~10MB) and the Whisper model (~250MB).
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
from openwakeword.model import Model as WakeModel

SAMPLE_RATE = 16000
CHUNK_MS    = 80           # openWakeWord requires 80ms chunks (1280 samples @ 16kHz)
CHUNK_SIZE  = int(SAMPLE_RATE * CHUNK_MS / 1000)  # 1280 samples

MAX_COMMAND_SECONDS     = 10
SILENCE_COMMAND_SECONDS = 1.0

PIPE_PATH     = "/tmp/homeassistant.pipe"
OWW_MODEL     = "hey_jarvis"
OWW_THRESHOLD = 0.5
MPV_SOCKET    = "/tmp/ha-mpv.sock"
DUCK_VOLUME   = 5

# ---------------------------------------------------------------------------
# Model initialisation (loaded once at startup)
# ---------------------------------------------------------------------------

vad_model, _ = torch.hub.load(
    'snakers4/silero-vad', 'silero_vad', force_reload=False, trust_repo=True
)

oww = WakeModel(wakeword_models=[OWW_MODEL], vad_threshold=0.5)

whisper = WhisperModel(
    "Systran/faster-distil-whisper-small.en",
    device="cpu",
    compute_type="int8",
    cpu_threads=4,
)

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
    while True:
        try:
            return open(path, 'w')
        except (FileNotFoundError, OSError) as e:
            print(f"Pipe not ready ({e}), retrying in 2s...")
            time.sleep(2)


def vad_score(chunk_np):
    """Run Silero VAD over two 512-sample sub-chunks from an 80ms chunk, return max."""
    scores = []
    for start in (0, 512):
        sub = chunk_np[start:start + 512]
        if len(sub) == 512:
            scores.append(vad_model(torch.tensor(sub), SAMPLE_RATE).item())
    return max(scores) if scores else 0.0


def record_until_silence(stream, device_rate, silence_seconds, max_seconds, initial_chunk=None):
    """Record chunks until Silero VAD detects sustained silence."""
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

def transcribe(audio_np):
    segments, _ = whisper.transcribe(
        audio_np,
        language="en",
        beam_size=1,
        initial_prompt=_COMMAND_PROMPT,
        vad_filter=True,
    )
    return " ".join(s.text.strip() for s in segments).strip()


def strip_wake_phrase(text):
    """Strip wake word from transcript if present (user may speak command in one breath)."""
    lower = text.lower().strip()
    idx = lower.find(OWW_MODEL.replace("_", " "))
    if idx != -1:
        return text[idx + len(OWW_MODEL):].strip(" ,.")
    return text


def _mpv_command(command):
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
    prev = _mpv_command(["get_property", "volume"])
    if prev is not None:
        _mpv_command(["set_property", "volume", DUCK_VOLUME])
    return prev


def unduck_volume(prev):
    if prev is not None:
        _mpv_command(["set_property", "volume", prev])


_GRAY   = "\033[90m"
_YELLOW = "\033[33m"
_GREEN  = "\033[32m"
_CYAN   = "\033[36m"
_RED    = "\033[31m"
_RESET  = "\033[0m"

def status(msg):
    print(msg, flush=True)


def write_to_pipe(pipe_fd, text, pipe_path):
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
    parser = argparse.ArgumentParser(description="Voice listener for Home Assistant Router")
    parser.add_argument("--device", type=int, default=None,
                        help="sounddevice input device index (default: system default)")
    parser.add_argument("--pipe", default=PIPE_PATH,
                        help=f"FIFO path (default: {PIPE_PATH})")
    parser.add_argument("--list-devices", action="store_true",
                        help="print available input devices and exit")
    args = parser.parse_args()

    if args.list_devices:
        for i, dev in enumerate(sd.query_devices()):
            if dev['max_input_channels'] > 0:
                print(f"  [{i}] {dev['name']}")
        sys.exit(0)

    pipe_fd = open_pipe_with_retry(args.pipe)

    device_info  = sd.query_devices(args.device, 'input')
    device_rate  = int(device_info['default_samplerate'])
    device_chunk = int(device_rate * CHUNK_MS / 1000)
    print(f"Device: {device_rate}Hz → {SAMPLE_RATE}Hz  |  Wake word: \"{OWW_MODEL}\"")

    with sd.InputStream(
        samplerate=device_rate, channels=1, dtype='int16',
        blocksize=device_chunk, device=args.device,
    ) as stream:
        status(f"{_GRAY}Listening for wake word... (Ctrl+C to quit){_RESET}")
        while True:
            chunk, _ = stream.read(device_chunk)
            chunk_np = resample_to_model(chunk, device_rate)

            scores    = oww.predict(chunk_np)
            score     = scores.get(OWW_MODEL, 0.0)
            rms       = float(np.sqrt(np.mean(chunk_np ** 2)))
            vol_bars  = int(min(rms * 400, 30))
            wake_bars = int(score * 10)

            print(
                f"\r{_GRAY}🎙  [{'█' * vol_bars:<30}]  wake:{score:.2f} [{'█' * wake_bars:<10}]{_RESET}",
                end="", flush=True,
            )

            if score < OWW_THRESHOLD:
                continue

            oww.reset()
            print()

            prev_vol = duck_volume()
            try:
                status(f"{_GREEN}✓  Wake word!  Recording command...{_RESET}")
                t0 = time.monotonic()
                cmd_audio = record_until_silence(
                    stream, device_rate,
                    silence_seconds=SILENCE_COMMAND_SECONDS,
                    max_seconds=MAX_COMMAND_SECONDS,
                )
                t_rec = time.monotonic()
                status(f"{_YELLOW}⏳  Transcribing...  (recorded {t_rec - t0:.1f}s){_RESET}")
                transcript = transcribe(cmd_audio)
                t_tr = time.monotonic()

                command = strip_wake_phrase(transcript)
                if command:
                    status(f"{_CYAN}▶  {command}  (transcribe {t_tr - t_rec:.1f}s){_RESET}")
                    pipe_fd = write_to_pipe(pipe_fd, command, args.pipe)
                else:
                    status(f"{_RED}✗  No command heard  (transcribe {t_tr - t_rec:.1f}s){_RESET}")
            finally:
                unduck_volume(prev_vol)

            status(f"{_GRAY}Listening for wake word... (Ctrl+C to quit){_RESET}")


if __name__ == "__main__":
    main()
