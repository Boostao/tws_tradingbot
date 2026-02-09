#!/usr/bin/env bash
set -euo pipefail

export DEBIAN_FRONTEND=noninteractive

apt-get update
apt-get install -y --no-install-recommends \
  ca-certificates \
  curl \
  unzip \
  jq \
  openjdk-17-jre-headless \
  xvfb \
  libxrender1 \
  libxtst6 \
  libxi6 \
  libxext6 \
  libxrandr2 \
  libxfixes3 \
  libxdamage1 \
  libxcomposite1 \
  libxcursor1 \
  libxinerama1 \
  libxkbcommon0 \
  libnss3 \
  libasound2t64

mkdir -p /opt/ibgateway /opt/ibc

download() {
  local url="$1"
  local dest="$2"
  echo "Downloading: $url"
  curl -fL --retry 3 --retry-delay 2 --connect-timeout 15 -o "$dest" "$url"
}

IBG_INSTALLER=/tmp/ibgateway-installer.sh
IBG_URLS=(
  "https://download2.interactivebrokers.com/installers/ibgateway/latest-standalone/ibgateway-latest-standalone-linux-x64.sh"
  "https://download2.interactivebrokers.com/installers/ibgateway/ibgateway-latest-standalone-linux-x64.sh"
  "https://download2.interactivebrokers.com/installers/ibgateway/stable/ibgateway-latest-standalone-linux-x64.sh"
)
IBG_DOWNLOADED=false
for url in "${IBG_URLS[@]}"; do
  if download "$url" "$IBG_INSTALLER"; then
    IBG_DOWNLOADED=true
    break
  fi
done
if [[ "$IBG_DOWNLOADED" != "true" ]]; then
  echo "Failed to download IB Gateway installer." >&2
  exit 1
fi
chmod +x "$IBG_INSTALLER"
"$IBG_INSTALLER" -q -dir /opt/ibgateway

IBC_API_URL="https://api.github.com/repos/IbcAlpha/IBC/releases/latest"
IBC_RELEASE_JSON=$(curl -fsSL -H "Accept: application/vnd.github+json" -H "User-Agent: ibc-install" "$IBC_API_URL")
IBC_ZIP_URL=$(echo "$IBC_RELEASE_JSON" | jq -r '.assets[] | select(.name | test("IBCLinux.*zip$")) | .browser_download_url' | head -n 1)
IBC_TAG=$(echo "$IBC_RELEASE_JSON" | jq -r '.tag_name')
if [[ -z "$IBC_ZIP_URL" && -n "$IBC_TAG" && "$IBC_TAG" != "null" ]]; then
  IBC_ZIP_URL="https://github.com/IbcAlpha/IBC/releases/download/${IBC_TAG}/IBCLinux-${IBC_TAG#v}.zip"
fi
if [[ -z "$IBC_ZIP_URL" ]]; then
  echo "Failed to resolve latest IBC download URL." >&2
  exit 1
fi

IBC_ZIP=/tmp/ibc.zip
download "$IBC_ZIP_URL" "$IBC_ZIP"
unzip -q -o "$IBC_ZIP" -d /opt/ibc

IBC_START=$(find /opt/ibc -type f -name ibcstart.sh -print -quit || true)
if [[ -z "$IBC_START" ]]; then
  echo "ibcstart.sh not found under /opt/ibc." >&2
  exit 1
fi

chmod +x "$IBC_START"

echo "IB Gateway installed to /opt/ibgateway"
echo "IBC installed; start script at: $IBC_START"
