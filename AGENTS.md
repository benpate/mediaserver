# Media Server — Notes for AI Agents

- **Three filesystems, three roles.** `original` holds untouched uploads, `processed` is a regenerable cache of transcoded results (safe to wipe), and `working` is a local temp area with a TTL. `Serve` walks original → processed → working on demand, generating each layer only when it is missing.

- **Cover-image fetching is SSRF-hardened.** `FileSpec.Metadata["cover"]` is an arbitrary URL that the server downloads. It goes through an SSRF-guarded [remote](https://github.com/benpate/remote) client: only `http`/`https` schemes are allowed, private/loopback IPs are blocked by default (opt in with `WithAllowPrivateIPs`), an optional host allow-list is enforced (`WithAllowedHosts`), and the body is size-capped. FFmpeg then reads only the local file, with `-protocol_whitelist file` so a disguised playlist can't reach back out.

- **A missing or blocked cover is not fatal.** If the cover download fails, it is logged and the media is processed without art — it never aborts the request.

- **The `working` directory must be `Close`d.** `NewWorkingDirectory` launches a background eviction goroutine; failing to `Close` it leaks the goroutine and leaves temp files behind.

- **Working-file removal deletes from disk directly, on purpose.** Otter notifies its deletion listener asynchronously, so `Remove`, `RemoveAll`, and `RemoveByOriginal` call `os.Remove` themselves rather than relying on `onDelete`. Routing them back through the listener would silently reintroduce two bugs: `Close` (which clears the cache and then shuts the processor down) would strand every temp file, and a deleted file would stay servable until the buffer drained.

- **Working filenames must be contained by the working folder.** `WorkingDirectory.filename` rejects anything `filepath.IsLocal` refuses, because `filepath.Join` *cleans* `../escape` into a real path outside the folder instead of failing. `FileSpec.Filename` is caller-supplied, so this is the only thing standing between an untrusted name and an arbitrary write.

- **`isWorkingFileFor` must stay stricter than `HasPrefix`.** A working name is `Filename` + `_args` + `.ext`, so a plain prefix test would let `abc` match `abcdef.webp` and delete or serve the wrong file.

- **`FileSpec.Cache` is exported, documented, and read by nothing.** `Serve` always goes through the processed cache and the working directory, so the field has no effect. Emissary believes otherwise — `build/step_ViewAttachment.go` assigns it from the template's `cache:` argument and `model/attachmentRules.go` hard-codes it true — which means a template asking for `cache: false` is silently cached anyway. Implementing it is a design decision, not a mechanical fix: `Cache: false` could mean bypass the processed cache, bypass the working directory, both, or change the `Cache-Control`/`ETag` headers, and guessing risks invalidating existing cache keys. If the answer is that the field was a mistake, mark it `// Deprecated:` rather than deleting it, and drop the Emissary assignment.

- **FFmpeg is required for media transforms.** Non-media files are copied through verbatim, but image/audio/video processing fails cleanly if `ffmpeg` is not on the `PATH`.

- **A failed request must leave nothing in the processed cache.** `ensureProcessedFileExists` creates the cache file before it processes into it, so a failure has to remove it, by `filespec.ProcessedPath()` and never by the open file's `Name()`. Emissary layers the cache as a `BasePathFs` for the storage location under another for the hostname, and through that `Name()` keeps a leading slash the outer layer never strips, so a removal by `Name()` misses and the empty file is served as the result forever. For the same reason a zero-length cache file counts as missing. The in-memory filesystems most tests use cannot show this; `newNestedServer` in [mediaserver_processedCache_test.go](mediaserver_processedCache_test.go) builds the real layering.
