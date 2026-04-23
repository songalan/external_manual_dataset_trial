#!/bin/bash
set -e
ISSUE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Activate conda environment if available
if conda env list | grep -q "^dbmate-status "; then
    eval "$(conda shell.bash hook)"
    conda activate dbmate-status
fi

# Source environment variables if set
[ -f "$HOME/.issue_env" ] && source "$HOME/.issue_env"

# Run Python verifier
python3 "$ISSUE_DIR/tests/test_outputs.py"
