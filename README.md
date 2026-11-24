<p align="center">
  <img src="assets/banner.svg" width="310" alt="netwhy: which layer broke?">
</p>

<p align="center">
  Find out why you can't reach something. Tests every layer from link to HTTP,
  names the layer that failed, and shows the evidence.
</p>

<p align="center">
  <a href="https://github.com/rahulkaushal04/netwhy/actions/workflows/ci.yml"><img src="https://github.com/rahulkaushal04/netwhy/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://pkg.go.dev/github.com/rahulkaushal04/netwhy"><img src="https://pkg.go.dev/badge/github.com/rahulkaushal04/netwhy.svg" alt="Go Reference"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-Apache--2.0-blue" alt="License"></a>
</p>

## Install

```sh
go install github.com/rahulkaushal04/netwhy/cmd/netwhy@latest
```

Or download a binary from the [Releases](https://github.com/rahulkaushal04/netwhy/releases) page.

## Usage

```sh
netwhy https://example.com
netwhy example.com:22
netwhy --timeout 2s 10.0.0.5:443
netwhy --markdown example.com      # paste into a ticket
netwhy --json example.com          # for scripts and CI health checks
netwhy watch --interval 30s example.com
```

A bare host is tested as `https` on port 443. `host:port` stops after the path
MTU check. `--timeout` is the time budget for each probe (default 5s).
`watch` re-checks on an interval and prints a line only when the result
changes.

The JSON output has a `schema_version` (currently 1); its fields are listed in
the doc comment of `render.JSON`.

When something is broken, netwhy names the lowest failing layer and shows its
evidence:

```
netwhy: https://no-such-host-xyz.invalid

  ✓ system   interface up, route out via 192.168.1.8:51032
  ✓ gateway  gateway ARP check is only available on Linux, skipped
  ✗ dns      no-such-host-xyz.invalid does not exist (NXDOMAIN from every resolver)
  - portal   skipped because dns did not work
  - tcp      skipped because portal did not work
  - pmtu     skipped because tcp did not work
  - tls      skipped because tcp did not work
  - http     skipped because tls did not work

Verdict: the dns layer failed
  no-such-host-xyz.invalid does not exist (NXDOMAIN from every resolver)

Evidence:
  sent:     A and AAAA for no-such-host-xyz.invalid to [fe80::6e4f:89ff:feb6:2519%en0]:53, 192.168.1.1:53
  received: [fe80::6e4f:89ff:feb6:2519%en0]:53: NXDOMAIN, 0 address(es), 14ms
            192.168.1.1:53: NXDOMAIN, 0 address(es), 15ms
  took:     24ms
```

Layers that work but are degraded, such as IPv6 failing while IPv4 works or a
certificate that expires soon, are shown as warnings (`!`) and do not change
the exit code.

Exit codes: `0` no layer failed, `1` a layer failed, `2` bad input.

## How it works

netwhy runs probes in dependency order: **system → gateway → dns → portal →
tcp → pmtu → tls → http**. A probe whose dependency did not work is skipped, so one fault gives
one failure with its evidence instead of a pile of errors. The verdict is the
lowest failing layer.

| Layer   | Checks |
|---------|--------|
| system  | an active interface with a usable IP, and a route out |
| gateway | the default gateway is in the ARP table with a hardware address (Linux only) |
| dns     | asks every configured resolver directly for A and AAAA, over UDP with a TCP retry. Warns on NXDOMAIN hijacking, a resolver answering 0.0.0.0, or private answers from one resolver only |
| portal  | fetches a URL that must answer 204; anything else means a captive portal or filtering proxy |
| tcp     | connects over IPv4 and IPv6 at the same time; refused, timed out and unreachable are told apart |
| pmtu    | ICMP echoes with the don't-fragment bit find the path MTU; warns when it is below the interface MTU (IPv4) |
| tls     | handshake, certificate chain, hostname match and expiry (https targets) |
| http    | one GET, reading 64 KiB of the body, with a timing breakdown. Warns when the body stalls (a PMTU black hole) or a proxy is configured |

## Limits

- The gateway check needs the Linux neighbour table. macOS does not show it to
  unsigned programs, so the probe passes there and says it was skipped.
- The portal check requests `connectivitycheck.gstatic.com`, the only request
  to a host you did not name. Skip it with `--no-portal`.
- The pmtu check needs unprivileged ICMP sockets (on Linux, set
  `net.ipv4.ping_group_range`) and a target that answers ping. Otherwise it
  passes and says it was not measured.
- DNS queries do not apply search domains, `/etc/hosts` or mDNS. If no
  resolver answers but the system resolver does, netwhy warns.
- Proxy environment variables are not used. A configured proxy is reported as
  a warning.

## Development

```sh
go test -race ./...
sudo test/netns/run.sh "$PWD/netwhy"   # Linux only: breaks one thing per scenario
```

## License

Apache-2.0, see [LICENSE](LICENSE).
