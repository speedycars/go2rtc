# Migrated Nest Hello (1st Gen) → Google Home → go2rtc → Frigate → Home Assistant

> Tested community deployment guide — 28 October 2026. This is a specialized fork of [AlexxIT/go2rtc](https://github.com/AlexxIT/go2rtc), not an official Google, Nest, Frigate, or go2rtc release. "Stable" means stable in the configuration described below; compatibility with other Nest generations and future SDM changes has not been established.

## Project provenance and acknowledgements

This working configuration builds on **[AlexxIT/go2rtc](https://github.com/AlexxIT/go2rtc)** and the related Nest/WebRTC work in **[@ajplotkin's fork](https://github.com/ajplotkin/go2rtc)**. We specifically thank **@ajplotkin** for his involvement in H.264 RTP reorder/re-anchor fixes, reviewing the changes, and multi-camera soak testing that helped validate the approach. This fork adds H.264 RTX SDP negotiation, focused delivery diagnostics, and a Frigate configuration validated with one migrated first-generation Nest Hello. For exact code authorship, consult the Git history and merged pull requests. Keep the existing license and notices when redistributing binaries or source.

## What we achieved

A first-generation wired **Nest Hello** was migrated from the original Nest app to **Google Home**. The Google Smart Device Management (SDM) API now provides a **WebRTC** live stream, with **H.264 video and Opus audio**. This is materially different from older Nest-app **RTSP/AAC** setups: users must inspect their device's `CameraLiveStream.supportedProtocols` rather than hard-code RTSP or expect the original AAC track.

The deployment uses a custom go2rtc binary **inside Frigate**, with a persistent `/config` directory bind-mounted from the Docker host. Frigate records video and performs object detection. Its official integration exposes camera/live view, detection events, recordings, and other supported entities to **Home Assistant**. The user retains the Google Home app for camera management but relies on **Frigate for self-hosted recording and history** instead of a Nest Aware video-recording subscription.

**Cost/architecture disclaimer:** A Nest Aware subscription is **not required for this tested live stream / self-hosted NVR workflow**. Google Device Access typically requires a **one-time US$5 registration** and might involve separately billable Google Cloud services, depending on setup and usage. **Google SDM, OAuth, signaling/session extension, and access to the camera depend on Google's services and the Internet**; only the Frigate NVR storage/analysis part is local. No unsupported assertion of 100% local Nest connectivity is intended.

```
Nest Hello (1st gen, migrated)
         │ Google Home / Google SDM API
         ▼
  Google WebRTC (H.264 + Opus)
         │ RTX + H.264 RTP reorder recovery
         ▼
  Custom go2rtc inside Frigate
         │                         ├── Direct WebRTC live view (Opus)
         ├── RTSP restream          └── MSE-compatible AAC live view
         ▼
  Frigate 0.18.1 (record H.264 copy, encode AAC; detect 5 FPS)
         │
         ├── Local recordings / events
         └── Official Frigate integration + MQTT → Home Assistant
```

## Scope and prerequisites

- **Verified camera:** wired **Nest Hello / first-generation Nest Doorbell**, **after migration to Google Home**, with SDM `WEB_RTC` support.
- **Verified application:** Frigate **0.18.1-ac9ac50** (October 2026), custom go2rtc 1.9.14-derived fork, Docker with a host-mounted `/config`, Home Assistant with the **Frigate integration**.
- Google Device Access project, OAuth client credentials, SDM project ID, camera device ID, and a valid refresh token. See [Google Device Access](https://developers.google.com/nest/device-access) and the [Home Assistant Nest integration](https://www.home-assistant.io/integrations/nest/) for prerequisites. Google may change its setup flow.
- Working Frigate configuration, accessible Google APIs and Nest camera. For Home Assistant integration, a working MQTT broker is generally required for Frigate entities.
- Host architecture **Linux/amd64** if using the provided Linux/amd64 build. Do not assume this binary runs on ARM or other architectures.

**Secrets:** SDM client secrets and refresh tokens grant sensitive access. Do not put real values in README, Git, release artifacts, issue logs, or screenshots. Use container environment variables and restrictive `.env` permissions. Be careful: some go2rtc versions may expose credential-bearing `nest:?` URLs in diagnostics and `/api/streams`.

## Example Frigate configuration

The following is an **illustrative configuration based on the validated pipeline**. Merge it into your existing Frigate `config.yml`, do not replace unrelated settings (MQTT, detection backend, zones, storage, other cameras). Check the current configuration schema in your own Frigate version.

```yaml
# Frigate config.yml (relevant sections only)

go2rtc:
  preload:
    nest_hello: "video=h264&audio=opus"
  streams:
    nest_hello:
      - "nest:?client_id={FRIGATE_NEST_CLIENT_ID}&client_secret={FRIGATE_NEST_CLIENT_SECRET}&refresh_token={FRIGATE_NEST_REFRESH_TOKEN}&project_id={FRIGATE_NEST_PROJECT_ID}&device_id={FRIGATE_NEST_DEVICE_ID}&protocols=WEB_RTC"

    # Separate source for browser-friendly MSE audio. In our deployed setup
    # direct WebRTC uses Opus, while the MSE source provides AAC on demand.
    nest_hello_mse:
      - "rtsp://127.0.0.1:8554/nest_hello"
      - "ffmpeg:nest_hello#audio=aac"

cameras:
  nest_hello:
    enabled: true
    ffmpeg:
      inputs:
        # Important: preserve source RTP timestamps. Wallclock PTS caused
        # 200-400 ms stutters at IDR frames in our measurements.
        - path: rtsp://127.0.0.1:8554/nest_hello
          input_args: -avoid_negative_ts make_zero -fflags +genpts+discardcorrupt -rtsp_transport tcp -timeout 10000000
          roles:
            - record
        - path: rtsp://127.0.0.1:8554/nest_hello
          input_args: preset-rtsp-restream-low-latency
          roles:
            - detect
      hwaccel_args: preset-vaapi  # Linux Intel GPU; adapt/remove for other hosts
      output_args:
        record: preset-record-generic-audio-aac
    detect:
      width: 800
      height: 600
      fps: 5
    live:
      streams:
        "Stable MSE (AAC)": nest_hello_mse
        "Direct WebRTC (Opus)": nest_hello
```

**Note on config parsing:** The `nest:?` URL must use the camera's actual SDM `WEB_RTC` capability. Container environment-variable substitution in `go2rtc` is distinct from Frigate's own `environment_vars` setting: supply these variables to the **Docker container environment**. Percent-encode any credentials containing reserved URL characters (particularly `/`, `&`, `+`, `#`, `?`, `=`), using the appropriate URL query-value encoding. Do not blindly paste token strings into URLs.

### Why two inputs and two live-view choices?

Frigate's `detect` role can use the low-latency preset and downsample/resize to 5 FPS; this is not the recording frame rate. The `record` role copies the incoming 15-FPS H.264 video (`-c:v copy`) and transcodes **Opus to AAC** (`-c:a aac`) for MP4 compatibility. The separate `nest_hello_mse` live stream makes AAC available for MSE/Firefox, while direct WebRTC retains Opus. `live.streams` names are selectable in the Frigate UI. The exact go2rtc MSE source may need adjustment to avoid duplicate/unwanted tracks depending on your go2rtc revision; verify the codecs shown in go2rtc's stream inspection UI before concluding audio support works.

## Install custom go2rtc binary in Dockerized Frigate

Frigate supports a custom go2rtc executable at **`/config/go2rtc`**; this takes precedence over the bundled binary when the container is restarted. A bind mount makes the replacement persist when recreating/updating the container. The binary must be named `go2rtc` and be executable by the go2rtc service account (`chmod 755`). [Frigate documentation](https://docs.frigate.video/configuration/advanced/system/).

**Example host layout:**

```text
/home/youruser/vm/frigate/
├── config/
│   ├── config.yml
│   └── go2rtc              # executable, custom Linux/amd64 build
└── media/                  # Frigate recordings, independent persistent mount
```

The existing Docker Compose **Frigate service** should already bind mount `/config` (and ideally `/media/frigate`):

```yaml
services:
  frigate:
    image: ghcr.io/blakeblackshear/frigate:stable # use your tested/pinned image
    container_name: frigate
    restart: unless-stopped
    volumes:
      - /home/youruser/vm/frigate/config:/config
      - /home/youruser/vm/frigate/media:/media/frigate
    environment:
      FRIGATE_NEST_CLIENT_ID: "${FRIGATE_NEST_CLIENT_ID}"
      FRIGATE_NEST_CLIENT_SECRET: "${FRIGATE_NEST_CLIENT_SECRET}"
      FRIGATE_NEST_REFRESH_TOKEN: "${FRIGATE_NEST_REFRESH_TOKEN}"
      FRIGATE_NEST_PROJECT_ID: "${FRIGATE_NEST_PROJECT_ID}"
      FRIGATE_NEST_DEVICE_ID: "${FRIGATE_NEST_DEVICE_ID}"
    # Add the rest of YOUR existing Frigate ports, shm_size, devices,
    # detector, network, privileged/capabilities, and other settings.
```

This **is not a complete Frigate Compose deployment**: preserve the hardware, detector, port, `shm_size`, and other settings from your current working service. Set secrets using a protected environment (`.env` ignored by Git, `chmod 600`) or another supported secret manager. Avoid exposing port 1984 or 5000 unauthenticated to the public Internet.

**Safe binary replacement** (only after release binary checksum, architecture and behavior are verified):

```bash
HOST_CONFIG=/home/youruser/vm/frigate/config
NEW_BINARY=/path/to/downloaded/release/go2rtc-linux-amd64

sha256sum "$NEW_BINARY"
file "$NEW_BINARY"   # expect Linux x86-64 ELF
"$NEW_BINARY" -version

# Back up the running known-good custom build, do NOT overwrite without backup.
cp -a "$HOST_CONFIG/go2rtc" "$HOST_CONFIG/go2rtc.backup.$(date +%Y%m%d-%H%M%S)"
install -m 755 "$NEW_BINARY" "$HOST_CONFIG/go2rtc.candidate"
mv -f "$HOST_CONFIG/go2rtc.candidate" "$HOST_CONFIG/go2rtc"

# Only once you are ready for a short service interruption:
docker restart --time 30 frigate

docker exec frigate /config/go2rtc -version
docker inspect frigate --format 'Status={{.State.Status}} Health={{.State.Health.Status}}'
```

If the Frigate deployment is managed by Compose, keep using your existing Compose lifecycle for image updates; the executable remains on the host under the persistent `/config`. An already running process will not switch binaries until restarted. If something fails, restore the backup to `config/go2rtc`, ensure mode 755, and restart Frigate.

### If building from source

```bash
git checkout <verified-release-tag>
go version
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -ldflags='-s -w' -o go2rtc-linux-amd64 .
sha256sum go2rtc-linux-amd64
```

**Release integrity is essential:** the initial tested RTX executable reported `1.9.14+dev.51c3392.dirty`: it was built from an **uncommitted working tree**. The stable release tag MUST contain that same source-level RTX implementation, reorder behavior, diagnostics and tests; simply tagging base commit `51c3392` is **not sufficient**. Rebuilding from a newly committed tag may legitimately change the SHA-256; test the tagged build before attaching it to a release. Never claim that an untested build is byte-identical to the validated production binary.

## Findings from the October 2026 troubleshooting

| Test/configuration | Observed behavior | Decision |
|---|---|---|
| Nest app (legacy RTSP/AAC) configuration after Google Home migration | API stream format changed to WebRTC / Opus | Use SDM `protocols=WEB_RTC`; do not expect legacy RTSP mode |
| Baseline go2rtc without negotiated RTX | H.264 packet loss produced visible macroblocking and incomplete recovery | Implement/enable SDP-negotiated RTX with Pion and preserve reorder recovery |
| Modified go2rtc, RTX registered in SDP | Successful RTX negotiation, packets delivered; waving-hand test substantially cleaner | Retain custom go2rtc RTX build |
| `preset-rtsp-restream-low-latency` for record | Earlier long-running Frigate exports showed audio lag (about 8 s) | Not selected as the final record preset |
| `preset-rtsp-restream` for record | Did not reliably resolve earlier audio synchronization issue | Not chosen as final record preset |
| `preset-rtsp-generic` for record | Audio appeared synchronized, but `-use_wallclock_as_timestamps 1` produced 200-400 ms frame pacing spikes near IDR frames and AAC non-monotonic DTS warnings | Avoid wallclock timestamps |
| Custom generic-like input arguments **without wallclock**; AAC recording | Smooth 15 FPS video, near-zero intra-segment A/V timestamp offset; observed synchronized audio for multiple days | **Recommended, validated setup** |

### Reproducible timestamp A/B results

Two independent 120-second simultaneous FFmpeg recording pipelines were run against the same go2rtc RTSP source, with identical recording parameters (`-c:v copy -c:a aac -f segment -segment_time 10 -reset_timestamps 1`), changing only `-use_wallclock_as_timestamps`:

| Measured metric (12 MP4 segments each) | Native timestamps | Wallclock timestamps |
|---|---:|---:|
| Video PTS intervals > 200 ms | 0 | 22 |
| Video PTS intervals < 10 ms | 0 | 90 |
| Maximum inter-frame PTS interval | ~67.3 ms | ~384.2 ms |
| AAC `Non-monotonic DTS` warnings | 0 | 149 |
| Median start audio-minus-video timestamp offset | ~10 ms | ~39 ms |
| Median end audio-minus-video timestamp offset | ~10 ms | ~268 ms |

Measured timestamp offsets do **not**, by themselves, prove lip sync; this was also evaluated through actual recorded playback. The direct 120-second test does not imply that every model/version is immune to long-term drift. In our deployment, the final configuration was observed stable from Oct 25 to Oct 28, 2026. Two momentary irregularities near a day/night mode change were not conclusively linked to the camera, and are not presented as a verified bug fix.

### RTX diagnostics and remaining limitations

Pion WebRTC was configured to negotiate RTX payload types for H.264 (PT99 apt96, PT102 apt97, PT103 apt98), matching SDM's answer and FID primary/repair SSRCs. Our instrumented build reports `rtx_delivered`, `rtx_gap_head_filled`, `rtx_late_or_old`, `rtx_duplicate_pending`, `rtx_other`, `rtp_gap_events`, and `rtp_missing_total` (cumulative per connection). During an early 107-minute test, 7,383 RTX packets were delivered, 3,134 of them filled the currently missing reorder head, and 13 unrecovered gap events / 20 missing sequences were reported. This is **not a controlled A/B improvement percentage**; network conditions differ. `rtx_gap_head_filled` is a narrow counter, not total usefulness of retransmission. PLI/keyframe recovery was not changed in the final configuration. Remaining network loss or a camera day/night transition can still cause brief glitches.

## Home Assistant integration

Use the [official Frigate integration](https://docs.frigate.video/integrations/home-assistant/) (normally installed through HACS) on the Home Assistant instance. Configure MQTT consistently in both Home Assistant and Frigate for object/detection/status entities. Configure the Frigate integration URL and RTSP connectivity according to whether Frigate runs on another host, behind a reverse proxy, or within Home Assistant. This architecture exposes Frigate's stream, detections and local recording history to Home Assistant **without making the camera itself a fully local device**. Your Google Home app can remain the camera's native management interface.

## Minimal validation checklist

1. Confirm the custom binary is used: `docker exec frigate /config/go2rtc -version` and inspect Frigate go2rtc startup logs.
2. Confirm Frigate is healthy: `docker inspect frigate --format '{{.State.Health.Status}}'`.
3. Verify the `record` FFmpeg command includes `-fflags +genpts+discardcorrupt`, `-c:v copy`, `-c:a aac`, and **does not include** `-use_wallclock_as_timestamps 1`.
4. Verify the camera exposes live WebRTC (Opus), MSE (AAC), recordings and detection events. Check browser playback separately; Firefox needs an AAC-compatible MSE stream for audio.
5. Analyze several *new* recording segments with `/usr/lib/ffmpeg/8.0/bin/ffprobe` (path depends on your Frigate image): expected ~66.67 ms video frame intervals at 15 FPS, absent long spikes around every IDR.
6. Confirm content-level A/V sync using sound-and-motion events over a longer period, not only timestamps.
7. Check RTX diagnostics for sustained recovery / abandoned gaps, recognizing counters reset on new go2rtc sessions or container recreation.

## References

- [Google Device Access: legacy doorbell](https://developers.google.com/nest/device-access/api/doorbell)
- [Google Device Access: wired doorbell, WebRTC](https://developers.google.com/nest/device-access/api/doorbell-wired)
- [Home Assistant Nest integration / one-time Device Access fee](https://www.home-assistant.io/integrations/nest/)
- [Frigate custom go2rtc version](https://docs.frigate.video/configuration/advanced/system/)
- [Frigate live view and codec support](https://docs.frigate.video/configuration/live/)
- [Frigate official Home Assistant integration](https://docs.frigate.video/integrations/home-assistant/)
- [go2rtc Nest source](https://github.com/AlexxIT/go2rtc/blob/master/internal/nest/README.md)
- [ajplotkin go2rtc fork](https://github.com/ajplotkin/go2rtc)
