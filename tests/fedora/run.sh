#!/bin/sh
set -eu
test "$#" -eq 2
tests=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
export WIREPROXY_TEST_ROOT=$(mktemp -d)
trap 'rm -rf -- "$WIREPROXY_TEST_ROOT"' EXIT
export XDG_DATA_HOME="$WIREPROXY_TEST_ROOT/data"
mkdir -p "$XDG_DATA_HOME/dbus-1/services"
dbus-run-session -- sh "$tests/smoke.sh" "$1" "$2"
