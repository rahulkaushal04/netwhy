<p align="center">
  <img src="assets/banner.svg" width="310" alt="netwhy: which layer broke?">
</p>

<p align="center">
  Find out why you can't reach something. Tests every layer from link to TCP,
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
```

When something is broken:

```
netwhy: https://no-such-host-xyz.invalid

  ✓ system  interface up, route out via 192.168.1.20:54321
  ✗ dns     no-such-host-xyz.invalid does not exist (no such host)
  - tcp     skipped because dns did not pass

Verdict: the dns layer failed
  no-such-host-xyz.invalid does not exist (no such host)

Evidence:
  sent:     lookup no-such-host-xyz.invalid via system resolver
  error:    lookup no-such-host-xyz.invalid: no such host
  took:     12ms
```

Exit codes: `0` everything passed, `1` a layer failed, `2` bad input.

## How it works

netwhy runs probes in dependency order: **system → dns → tcp**. A probe whose
dependency failed is skipped, so you get one failure with its evidence instead
of a pile of errors. The verdict is the lowest failing layer.

| Layer  | Checks |
|--------|--------|
| system | an active interface with a usable IP, and a route out |
| dns    | the name resolves through the system resolver |
| tcp    | a connection opens; refused, timed out and unreachable are told apart |

## Status

Early (v0.1). Not yet done: direct per-resolver DNS queries, TLS and HTTP
checks, PMTU black-hole detection, `--json` output.

## License

Apache-2.0, see [LICENSE](LICENSE).