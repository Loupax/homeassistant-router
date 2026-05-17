#!/usr/bin/env python3
"""
Voice listener for Home Assistant Router.

Dependencies (pip):
    sounddevice
    numpy
    faster-whisper
    openwakeword
    torch torchaudio   (for silero-vad)

System packages (pacman):
    portaudio          (already installed)
    ffmpeg

Usage:
    python listener.py [--device INDEX] [--pipe /tmp/homeassistant.pipe]

Device index: run `python listener.py --list-devices` to see available inputs.

# Install dependencies:
#   pip install sounddevice numpy faster-whisper openwakeword torch torchaudio
#   sudo pacman -S portaudio ffmpeg
"""

import argparse
import os
import sys
import time

import numpy as np
import sounddevice as sd
import torch
from scipy.signal import resample_poly
from faster_whisper import WhisperModel
from openwakeword.model import Model

SAMPLE_RATE = 16000        # Hz — required by all three models
CHUNK_MS    = 80           # ms per chunk — openWakeWord requires multiples of 80ms
CHUNK_SIZE  = int(SAMPLE_RATE * CHUNK_MS / 1000)  # 1280 samples

MAX_RECORD_SECONDS = 10
SILENCE_THRESHOLD_SECONDS = 1.5
PIPE_PATH = "/tmp/homeassistant.pipe"

# ---------------------------------------------------------------------------
# Model initialisation (module level — loaded once at startup)
# ---------------------------------------------------------------------------

_OWW_MODEL_DIR = os.path.join(os.path.dirname(__import__("openwakeword").__file__), "resources/models")
_OWW_MODEL_KEY = "hey_jarvis_v0.1"
oww_model = Model(wakeword_model_paths=[os.path.join(_OWW_MODEL_DIR, f"{_OWW_MODEL_KEY}.onnx")])

vad_model, utils = torch.hub.load(
    'snakers4/silero-vad', 'silero_vad', force_reload=False, trust_repo=True
)
(get_speech_timestamps, _, read_audio, *_) = utils

whisper = WhisperModel("base.en", device="cpu", compute_type="int8")


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


def record_until_silence(stream, device_rate):
    """Record audio chunks until Silero VAD detects sustained silence."""
    buffer = []
    silence_chunks = 0
    device_chunk = int(device_rate * CHUNK_MS / 1000)
    max_chunks = int(MAX_RECORD_SECONDS * 1000 / CHUNK_MS)
    silence_limit = int(SILENCE_THRESHOLD_SECONDS * 1000 / CHUNK_MS)

    for _ in range(max_chunks):
        chunk, _ = stream.read(device_chunk)
        chunk_np = resample_to_model(chunk, device_rate)
        buffer.append(chunk_np)

        tensor = torch.tensor(chunk_np)
        speech_prob = vad_model(tensor, SAMPLE_RATE).item()

        if speech_prob < 0.5:
            silence_chunks += 1
            if silence_chunks >= silence_limit:
                break
        else:
            silence_chunks = 0

    return np.concatenate(buffer)


def transcribe(audio_np):
    """Transcribe audio using faster-whisper and return a single string."""
    segments, _ = whisper.transcribe(audio_np, language="en")
    return " ".join(s.text.strip() for s in segments).strip()


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

    # Use device's native sample rate; resample to MODEL_RATE for all models
    device_info = sd.query_devices(args.device, 'input')
    device_rate = int(device_info['default_samplerate'])
    device_chunk = int(device_rate * CHUNK_MS / 1000)
    print(f"Device sample rate: {device_rate}Hz (resampling to {SAMPLE_RATE}Hz)")

    with sd.InputStream(
        samplerate=device_rate,
        channels=1,
        dtype='int16',
        blocksize=device_chunk,
        device=args.device,
    ) as stream:
        print("Listening for wake word... (Ctrl+C to quit)")
        while True:
            chunk, _ = stream.read(device_chunk)
            chunk_np = resample_to_model(chunk, device_rate)

            rms = float(np.sqrt(np.mean(chunk_np ** 2)))
            bars = int(min(rms * 400, 30))

            oww_model.predict(chunk_np)
            scores = oww_model.prediction_buffer.get(_OWW_MODEL_KEY, [])
            score = scores[-1] if scores else 0.0
            score_bar = int(score * 10)
            print(f"\r🎙  [{'█' * bars:<30}]  wake:{score:.2f} [{'█' * score_bar:<10}]", end="", flush=True)

            if scores and scores[-1] > 0.5:
                print("\nWake word detected! Recording...")
                audio_buffer = record_until_silence(stream, device_rate)
                transcript = transcribe(audio_buffer)
                if transcript:
                    print(f"Transcribed: {transcript}")
                    pipe_fd = write_to_pipe(pipe_fd, transcript, args.pipe)
                print("Listening for wake word... (Ctrl+C to quit)")


if __name__ == "__main__":
    main()
