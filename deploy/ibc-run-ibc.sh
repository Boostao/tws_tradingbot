#!/bin/bash
set -euo pipefail

if [[ -z "${IKBR_USER:-}" || -z "${IKBR_PASSWORD:-}" ]]; then
  echo "IKBR_USER and IKBR_PASSWORD must be set in the environment." >&2
  exit 1
fi

mkdir -p /opt/ibc/ibc /opt/ibc/logs
sed \
  -e "s|__IB_USER__|${IKBR_USER}|" \
  -e "s|__IB_PASS__|${IKBR_PASSWORD}|" \
  /opt/ibc/ibc/config.template.ini > /opt/ibc/ibc/config.ini

chmod 600 /opt/ibc/ibc/config.ini
chown -R ibc:ibc /opt/ibc/ibc /opt/ibc/logs

exec su -s /bin/bash -c \
  "/usr/bin/xvfb-run -a /opt/ibc/IBC/scripts/ibcstart.sh 1019 --ibc-path=/opt/ibc/IBC --ibc-ini=/opt/ibc/ibc/config.ini --tws-path=/opt/ibc/tws --mode=paper --on2fatimeout=exit" \
  ibc
