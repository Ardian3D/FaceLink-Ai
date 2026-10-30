#!/usr/bin/env python3
"""Read-only queries against the FL-ENR06 disposable SQLite database."""
from __future__ import annotations

import json
import sqlite3
import sys
from pathlib import Path

if len(sys.argv) < 3:
    raise SystemExit("usage: dbq.py <db> <sql> [args...]")

path = Path(sys.argv[1]).resolve()
conn = sqlite3.connect(f"{path.as_uri()}?mode=ro", uri=True)
conn.row_factory = sqlite3.Row
try:
    rows = [dict(row) for row in conn.execute(sys.argv[2], sys.argv[3:])]
finally:
    conn.close()
print(json.dumps(rows))
