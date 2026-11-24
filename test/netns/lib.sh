#!/usr/bin/env bash
# Shared setup for the namespace scenarios. Sourced by run.sh, never run alone.
#
#   client 10.0.0.2 --- router 10.0.0.1 | 10.0.1.1 --- server 10.0.1.2
#
# The server runs dnsmasq (DNS) and openssl s_server (HTTPS) for $NAME.
# A scenario file sets the variables below and may define break_it().

NAME=test.netwhy.lab

# Defaults a scenario may override.
DESC=
EXPECT_LAYER=              # layer netwhy must name as failed; empty means none
EXPECT_TEXT=               # text the verdict summary must contain
EXPECT_WARN=               # probe that must be in the warnings
DNS_RULES="/$NAME/10.0.1.2" # dnsmasq address= rules
CERT_DAYS=30
CERT_NAME=$NAME

PIDS=()

cleanup() {
  local pid
  for pid in "${PIDS[@]}"; do kill "$pid" 2>/dev/null; done
  for ns in client router server; do ip netns del "nw-$ns" 2>/dev/null; done
  rm -rf /etc/netns/nw-client "$WORK"
}

setup_network() {
  local ns
  for ns in client router server; do
    ip netns add "nw-$ns"
    ip -n "nw-$ns" link set lo up
  done
  ip link add c0 type veth peer name r0
  ip link set c0 netns nw-client
  ip link set r0 netns nw-router
  ip link add r1 type veth peer name s0
  ip link set r1 netns nw-router
  ip link set s0 netns nw-server

  ip -n nw-client addr add 10.0.0.2/24 dev c0 && ip -n nw-client link set c0 up
  ip -n nw-router addr add 10.0.0.1/24 dev r0 && ip -n nw-router link set r0 up
  ip -n nw-router addr add 10.0.1.1/24 dev r1 && ip -n nw-router link set r1 up
  ip -n nw-server addr add 10.0.1.2/24 dev s0 && ip -n nw-server link set s0 up
  ip -n nw-client route add default via 10.0.0.1
  ip -n nw-server route add default via 10.0.1.1

  ip netns exec nw-router sysctl -qw net.ipv4.ip_forward=1
  # Unprivileged ICMP sockets, which the pmtu probe uses.
  ip netns exec nw-client sysctl -qw net.ipv4.ping_group_range="0 2147483647"

  # "ip netns exec" bind-mounts this file over /etc/resolv.conf.
  mkdir -p /etc/netns/nw-client
  echo "nameserver 10.0.1.2" >/etc/netns/nw-client/resolv.conf
}

make_cert() {
  openssl ecparam -genkey -name prime256v1 -noout -out "$WORK/key.pem"
  openssl req -new -key "$WORK/key.pem" -subj "/CN=$CERT_NAME" -out "$WORK/req.csr"
  openssl x509 -req -in "$WORK/req.csr" -signkey "$WORK/key.pem" -days "$CERT_DAYS" \
    -extfile <(printf 'subjectAltName=DNS:%s\n' "$CERT_NAME") -out "$WORK/cert.pem" 2>/dev/null
}

start_servers() {
  local args=() rule
  for rule in $DNS_RULES; do args+=("--address=$rule"); done
  ip netns exec nw-server dnsmasq --keep-in-foreground --no-hosts --no-resolv --conf-file=/dev/null \
    --pid-file= --user=root --listen-address=10.0.1.2 --bind-interfaces "${args[@]}" >"$WORK/dnsmasq.log" 2>&1 &
  PIDS+=($!)
  ip netns exec nw-server openssl s_server -accept 443 -cert "$WORK/cert.pem" -key "$WORK/key.pem" -www -quiet \
    >"$WORK/openssl.log" 2>&1 &
  PIDS+=($!)

  for _ in $(seq 50); do
    ip netns exec nw-server ss -lun | grep -q ':53 ' && ip netns exec nw-server ss -ltn | grep -q ':443 ' && return 0
    sleep 0.1
  done
  echo "servers did not start"; cat "$WORK/dnsmasq.log" "$WORK/openssl.log"
  return 1
}

fail() {
  echo "  $DESC: $1"
  [[ -n "$out" ]] && jq -r '.results[] | "    \(.name) \(.status) \(.summary)"' <<<"$out" 2>/dev/null
  return 1
}

check() {
  local out rc=0
  out=$(ip netns exec nw-client env SSL_CERT_FILE="$WORK/cert.pem" \
    "$NETWHY" --json --no-portal --timeout 3s "https://$NAME") || rc=$?
  [[ -n "$out" ]] || { echo "  $DESC: no output (exit $rc)"; return 1; }

  local layer text warns
  layer=$(jq -r '.layer // ""' <<<"$out")
  text=$(jq -r '.summary' <<<"$out")
  warns=$(jq -r '[.warnings[]?.name] | join(",")' <<<"$out")

  [[ "$layer" == "$EXPECT_LAYER" ]] || { fail "failed layer is '$layer', want '$EXPECT_LAYER'"; return 1; }
  if [[ -n "$EXPECT_TEXT" ]] && ! grep -qi -- "$EXPECT_TEXT" <<<"$text"; then
    fail "summary '$text' does not contain '$EXPECT_TEXT'"; return 1
  fi
  if [[ -n "$EXPECT_WARN" ]]; then
    [[ ",$warns," == *",$EXPECT_WARN,"* ]] || { fail "warnings '$warns' do not include '$EXPECT_WARN'"; return 1; }
  elif [[ -z "$EXPECT_LAYER" && -n "$warns" ]]; then
    fail "unexpected warnings: $warns"; return 1
  fi
}

run_scenario() {
  WORK=$(mktemp -d)
  trap cleanup EXIT
  setup_network
  make_cert
  start_servers || return 1
  if declare -F break_it >/dev/null; then break_it; fi
  check
}
