#!/bin/zsh
set -euo pipefail
exec /usr/bin/env python3 "${0:a:h}/build-restored-app.py" "$@"
