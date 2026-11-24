#!/usr/bin/env bash
# Runs each scenario in its own set of network namespaces and checks that
# netwhy names the right layer. Needs Linux, root, dnsmasq, nftables, openssl
# and jq.
#
#   sudo test/netns/run.sh /path/to/netwhy [scenario-name...]
set -u
cd "$(dirname "$0")" || exit 1

NETWHY=$(realpath "${1:?usage: run.sh /path/to/netwhy [scenario...]}")
shift
export NETWHY

if (($# > 0)); then
  scenarios=("$@")
else
  scenarios=()
  for f in scenarios/*.sh; do
    f=${f##*/}
    scenarios+=("${f%.sh}")
  done
fi

failed=0
for name in "${scenarios[@]}"; do
  # shellcheck disable=SC1091,SC1090
  if (source ./lib.sh && source "scenarios/$name.sh" && run_scenario); then
    echo "PASS $name"
  else
    echo "FAIL $name"
    failed=$((failed + 1))
  fi
done
echo "$failed scenario(s) failed"
((failed == 0))
