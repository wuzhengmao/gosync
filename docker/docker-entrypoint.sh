#!/bin/sh
set -e

INFO='\033[0;32m[INFO ] '
ERROR='\033[0;31m[ERROR] '
WARN='\033[1;33m[WARN ] '
NC='\033[0m'

_info()
{
  echo -e "${INFO}$@${NC}"
}

_warn()
{
  echo -e "${WARN}$@${NC}"
}

_error()
{
  echo -e "${ERROR}$@${NC}"
}

if [ -d "/docker-entrypoint.d" ] && [ "$(ls -A /docker-entrypoint.d)" ]; then
  _info "/docker-entrypoint.d/ is not empty, will attempt to perform configuration"
  _info "Looking for shell scripts in /docker-entrypoint.d/"
  find "/docker-entrypoint.d/" -follow -type f -print | sort -V | while read -r f; do
    case "$f" in
    *.sh)
      if [ ! -x "$f" ]; then
        chmod +x $f
      fi
      _info "Launching $f"
      "$f"
      ;;
    *)
      _info "Ignoring $f"
      ;;
    esac
  done
fi

if [ "$1" = "gosync" ]; then
  echo "Start gosync..."
  exec /usr/bin/gosync
else
  exec "$@"
fi