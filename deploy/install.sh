#!/usr/bin/env bash
# Install Core and Web from one release's Compose files. The host needs Docker.
set -euo pipefail

repository="${OAC_REPOSITORY:-MiniMax-AI/OpenAgentCore}"
version="latest"
install_dir="${OAC_INSTALL_DIR_DEFAULT:-$HOME/.oac/core}"
public_url=""
host_address="0.0.0.0"
web_port="8080"
allow_insecure_origin=0
kept=0

usage() {
  cat <<'EOF'
Usage: install.sh [--version TAG] [--install-dir DIR] [--public-url URL]
                  [--host ADDRESS] [--web-port PORT] [--allow-insecure-origin]

Installs Core, Web and PostgreSQL, and publishes Web on --web-port. HTTPS is
terminated by your reverse proxy or hosting platform. --allow-insecure-origin
permits a non-loopback plain-HTTP --public-url for development and testing.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version) version="${2:?}"; shift 2 ;;
    --install-dir) install_dir="${2:?}"; shift 2 ;;
    --public-url) public_url="${2:?}"; shift 2 ;;
    --host) host_address="${2:?}"; shift 2 ;;
    --web-port) web_port="${2:?}"; shift 2 ;;
    --allow-insecure-origin) allow_insecure_origin=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown argument: $1" >&2; usage >&2; exit 1 ;;
  esac
done

if [[ "$(uname -s)" != Linux || "$(uname -m)" != x86_64 ]]; then
  echo "Core installs on Linux amd64." >&2
  exit 1
fi
command -v docker >/dev/null || { echo "Docker Engine with Compose 2.26 or newer is required." >&2; exit 1; }
command -v curl >/dev/null || { echo "curl is required." >&2; exit 1; }
compose_version="$(docker compose version --short 2>/dev/null | sed 's/^v//' || true)"
major="${compose_version%%.*}"
minor="${compose_version#*.}"
minor="${minor%%.*}"
if [[ ! "$major" =~ ^[0-9]+$ || ! "$minor" =~ ^[0-9]+$ ]] || (( major < 2 || (major == 2 && minor < 26) )); then
  echo "Docker Compose 2.26 or newer is required (found ${compose_version:-none})." >&2
  exit 1
fi
if [[ "$install_dir" != /* ]]; then
  echo "--install-dir must be absolute." >&2
  exit 1
fi
if [[ -e "$install_dir" ]] && [[ -n "$(ls -A "$install_dir" 2>/dev/null || true)" ]]; then
  echo "Installation directory is not empty: $install_dir" >&2
  exit 1
fi

port_busy() {
  local port="$1"
  if command -v ss >/dev/null; then
    if ss -ltn | awk '{print $4}' | grep -Eq "(^|:|\\])${port}$"; then
      return 0
    fi
    return 1
  fi
  (echo >/dev/tcp/127.0.0.1/"$port") >/dev/null 2>&1
}
if port_busy "$web_port"; then echo "Port $web_port is already in use." >&2; exit 1; fi
asset_base="https://github.com/${repository}/releases/latest/download"
if [[ "$version" != latest ]]; then
  asset_base="https://github.com/${repository}/releases/download/${version}"
fi

cleanup() {
  if [[ "$kept" != 1 && -d "$install_dir" ]]; then
    (cd "$install_dir" && docker compose down --remove-orphans) >/dev/null 2>&1 || true
    rm -rf "$install_dir"
  fi
}
trap cleanup EXIT

mkdir -p "$install_dir"
chmod 700 "$install_dir"
files=(compose.yaml ports.yaml)
curl --fail --silent --show-error --location "$asset_base/compose-sha256sums.txt" --output "$install_dir/compose-sha256sums.txt"
for name in "${files[@]}"; do
  curl --fail --silent --show-error --location "$asset_base/$name" --output "$install_dir/$name"
done
(cd "$install_dir" && sha256sum --check --ignore-missing --quiet compose-sha256sums.txt)

compose_file="$(IFS=:; echo "${files[*]}")"
umask 077
{
  echo "COMPOSE_PROJECT_NAME=oac-$(od -An -N5 -tx1 /dev/urandom | tr -d ' \n')"
  echo "COMPOSE_FILE=$compose_file"
  echo "OAC_INSTALL_DIR=$install_dir"
  echo "OAC_HOST=$host_address"
  echo "OAC_WEB_PORT=$web_port"
  if [[ -n "$public_url" ]]; then echo "OAC_PUBLIC_URL=$public_url"; fi
  if [[ "$allow_insecure_origin" == 1 ]]; then echo "OAC_ALLOW_INSECURE_ORIGIN=1"; fi
} >"$install_dir/.env"

(
  cd "$install_dir"
  docker compose pull
  docker compose create core
  docker compose cp core:/usr/local/bin/oac ./oac
  chmod 755 ./oac
  # check-config reads the installation id and secrets that init writes, so
  # initialize the data directory before validating the settings. A rejected
  # configuration must fail here, not after Compose reports a running stack.
  if ! initialized="$(docker compose run --rm -T init 2>&1)"; then
    printf '%s\n' "$initialized" >&2
    echo "Installation initialization failed; no service was started." >&2
    exit 1
  fi
  if ! checked="$(docker compose run --rm -T --no-deps --entrypoint /usr/local/bin/oac-core core check-config 2>&1)"; then
    printf '%s\n' "$checked" >&2
    echo "Configuration check failed; no service was started." >&2
    exit 1
  fi
  docker compose up -d --wait
)
kept=1
trap - EXIT
address="http://${host_address}:$web_port"
if [[ -n "$public_url" ]]; then address="$public_url"; fi
if [[ "$host_address" == 0.0.0.0 || "$host_address" == "::" ]]; then address="http://<this-host>:$web_port"; fi
cat <<EOF
OpenAgentCore is running.
Console: $address
Core key: $install_dir/oac core-key --show
Manage the installation with $install_dir/oac.
EOF
