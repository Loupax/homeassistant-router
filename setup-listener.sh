#!/usr/bin/env bash
set -e

cd "$(dirname "$0")"

PYTHON="$HOME/.pyenv/versions/3.11.15/bin/python3.11"

if [ ! -x "$PYTHON" ]; then
    echo "Python 3.11 not found at $PYTHON"
    echo "Install with: pyenv install 3.11"
    exit 1
fi

"$PYTHON" -m venv .venv311
.venv311/bin/pip install --upgrade --quiet pip
.venv311/bin/pip install --quiet sounddevice "numpy<2" faster-whisper torch torchaudio scipy openwakeword

echo "Downloading openWakeWord models..."
.venv311/bin/python -c "import openwakeword; openwakeword.utils.download_models()"

echo ""
echo "Venv ready. Run with:"
echo "  .venv311/bin/python listener"
