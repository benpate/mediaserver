# Media Server — Notes for AI Agents

- **Three filesystems, three roles.** `original` holds untouched uploads, `processed` is a regenerable cache of transcoded results (safe to wipe), and `working` is a local temp area with a TTL. `Serve` walks original → processed → working on demand, generating each layer only when it is missing.

- **Cover-image fetching is SSRF-hardened.** `FileSpec.Metadata["cover"]` is an arbitrary URL that the server downloads. It goes through an SSRF-guarded [remote](https://github.com/benpate/remote) client: only `http`/`https` schemes are allowed, private/loopback IPs are blocked by default (opt in with `WithAllowPrivateIPs`), an optional host allow-list is enforced (`WithAllowedHosts`), and the body is size-capped. FFmpeg then reads only the local file, with `-protocol_whitelist file` so a disguised playlist can't reach back out.

- **A missing or blocked cover is not fatal.** If the cover download fails, it is logged and the media is processed without art — it never aborts the request.

- **The `working` directory must be `Close`d.** `NewWorkingDirectory` launches a background eviction goroutine; failing to `Close` it leaks the goroutine and leaves temp files behind.

- **FFmpeg is required for media transforms.** Non-media files are copied through verbatim, but image/audio/video processing fails cleanly if `ffmpeg` is not on the `PATH`.
