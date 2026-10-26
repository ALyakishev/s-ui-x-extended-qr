# S-UI-X Extended fork v1.1.2-tt.1 (prerelease)

First fork prerelease based on upstream `1.1.2-beta7`. Intended for controlled
TrustTunnel testing, including import of native `tt://` links into Shadowrocket.
A successful import and connection on the intended Shadowrocket version remains
an acceptance check; this release does not claim a completed device test.

## Fork changes

- Generate native TrustTunnel `tt://?` share links from configured inbound/client
  data using the binary TLV format and unpadded URL-safe Base64.
- Show links and QR codes through the existing panel UI. Support multiple server
  addresses, IPv4, IPv6, domains and non-default ports.
- Keep the connection address separate from TLS hostname/SNI. Use the selected
  TLS configuration and supported per-address overrides.
- Preserve TrustTunnel JSON client configuration and JSON subscriptions alongside
  native URI delivery; subscription metadata does not corrupt `tt://` payloads.
- Use Go 1.26.10, including the `crypto/x509` fix for GO-2026-6809.
- Fix synchronization and cleanup in the database restore regression test.
- Retain the existing embedded frontend and extended protocol build tags.

## Installation and verification

Use the **fork** assets from
https://github.com/ALyakishev/s-ui-x-extended-qr/releases/tag/v1.1.2-tt.1 .
For Debian 12 x86_64 select `s-ui-linux-amd64.tar.gz` and its adjacent SHA256 and
manifest. Linux binaries use static musl/CGO builds, including `with_trusttunnel`.

[Clean installation, offline backup, update and rollback](https://github.com/ALyakishev/s-ui-x-extended-qr/blob/v1.1.2-tt.1/docs/fork-trusttunnel-release.md).
Do not use upstream installation commands or panel/menu self-update for this
fork: those still download upstream binaries and can remove the fork features.
Back up the database, binary, encryption keys and systemd configuration before
updating. Never restore only an older binary onto a migrated database.

## Release assets

Linux: amd64, arm64, armv7, armv6, armv5, 386, s390x (archive, SHA256, manifest
for each). Windows: amd64 and arm64 (ZIP and SHA256 for each).
Shared: `s-ui-release.spdx.json` and `s-ui-release-attestation.json`, covering the
Linux and Windows archives. This is a prerelease and does not replace `latest`.
