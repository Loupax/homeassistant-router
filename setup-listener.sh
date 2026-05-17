#!/usr/bin/env bash
set -e

cd "$(dirname "$0")"

python3 -m venv .venv
.venv/bin/pip install --upgrade pip
.venv/bin/pip install -r requirements.txt

echo "Venv ready. Activate with: source .venv/bin/activate"
echo "Or run directly via: .venv/bin/python listener.py"
