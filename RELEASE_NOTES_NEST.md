# Nest WebRTC / RTX — tested Frigate integration release

**Release tag:** `v1.9.14-nest-rtx.1`
**Scope:** community-maintained go2rtc 1.9.14-derived build for a first-generation *wired Nest Hello migrated to Google Home*.

> This is a tested community-specific build, not a guarantee of universal Nest compatibility or permanent support for Google's API.

## Summary

Improves Google SDM WebRTC H.264 reception after legacy Nest Hello migration (legacy Nest RTSP/AAC → Google Home WebRTC/H.264/Opus), chiefly by enabling SDP-negotiated RTP retransmission (RTX) and retaining H.264 RTP reorder recovery. Tested with Frigate 0.18.1 and Home Assistant integration for self-hosted continuous recording and detection **without a Nest Aware subscription**. **Requires Google SDM/Device Access (normally a $5 one-time registration) and ongoing Google service/internet access.**

## Credits and provenance

This is a specialized community fork, **not an official upstream go2rtc release**.

- **[AlexxIT/go2rtc](https://github.com/AlexxIT/go2rtc)** — original go2rtc project and core streaming implementation.
- **[@ajplotkin](https://github.com/ajplotkin) / [ajplotkin/go2rtc](https://github.com/ajplotkin/go2rtc)** — important Nest/WebRTC development baseline, collaboration on H.264 RTP reorder/re-anchor behavior, code review, and extended multi-camera soak validation of the reorder changes. We thank him explicitly for these contributions.
- **[speedycars/go2rtc](https://github.com/speedycars/go2rtc)** — RTX negotiation/diagnostic changes and documentation of the verified Nest Hello → Frigate → Home Assistant deployment.

Individual commits and PR history are the authoritative record of code authorship; the above credits should not be interpreted as assigning every change to one person. Retain the existing upstream license and attribution.

## Code-level changes intended for this release

- Register RTX codecs for H.264 payload types 96/97/98 using RTX PT99/102/103 and apt mappings.
- Accept Google SDM RTX SDP answer and surface repaired packets to existing receiver/reorder logic.
- Retain tested H.264 reorder / stream continuity fixes; do not add periodic PLI or new watchdog behavior.
- Expose read-only diagnostics of RTX delivery and residual RTP gaps.
- Include focused test coverage for RTX registration and diagnostic counter categorization.

**Before publication:** Compare the release branch's complete source diff to the tested production checkout (`nestfix20-rtx-experiment`, previously based on `51c3392` with uncommitted changes), run the relevant Go tests and verify the release binary in a staging Frigate container. Do **not** tag base commit `51c3392` alone: the known-good executable was a `dirty` build.

## Production outcome (observed 25–28 Oct 2026)

- Consistent smooth ~15 FPS H.264 recording (tested 1600×1200 source), synced AAC audio and no systematic IDR-associated PTS spikes.
- Negotiated RTX substantially reduced visible macroblocking during movement in the tested installation; occasional unrecoverable RTP gaps remain possible.
- Frigate recording and Home Assistant integration operated successfully without Nest Aware during the observation period.

**Critical configuration detail:** The final working Frigate `record` input used:

```yaml
input_args: -avoid_negative_ts make_zero -fflags +genpts+discardcorrupt -rtsp_transport tcp -timeout 10000000
```

with `output_args.record: preset-record-generic-audio-aac`. The `detect` input used `preset-rtsp-restream-low-latency`. **Do not replace the custom record input with `preset-rtsp-generic`: its `-use_wallclock_as_timestamps 1` setting reproduced major frame-timestamp jitter and AAC DTS corrections in our A/B test.**

Detailed configuration, host bind-mount install, the 120-second A/B results, and limitations: [Nest / Google Home / Frigate / Home Assistant guide](docs/NEST_GOOGLE_HOME_FRIGATE.md).

## Assets to attach after validation

- `go2rtc-linux-amd64` (clean release build matching the tagged source)
- `SHA256SUMS` for the published release asset(s)

Do not attach credential-containing Frigate configs, log excerpts, OAuth tokens, or the old `dirty` binary without verifying its exact source and provenance.
