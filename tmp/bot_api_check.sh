#!/usr/bin/env bash
set -euo pipefail

echo "== Bot container -> API check =="
docker exec tws_tradingbot-bot-1 sh -lc 'cat > /tmp/api_check.py << "PY"
import urllib.request
import urllib.error

url = "http://api:8000/api/v1/state"
try:
    with urllib.request.urlopen(url, timeout=3) as resp:
        print(resp.status)
        body = resp.read(200)
        try:
            print(body.decode("utf-8", errors="replace"))
        except Exception:
            print(body)
except Exception as e:
    print("ERR", e)
PY
python /tmp/api_check.py'

echo

echo "== API container direct state =="
docker exec tws_tradingbot-api-1 sh -lc 'curl -sS --max-time 5 http://localhost:8000/api/v1/state | head -c 300'

echo
