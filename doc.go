// Package mediaserver stores uploaded media files and serves transformed
// versions of them on demand, using FFmpeg to resize images and transcode
// audio and video.
//
// A MediaServer works across three filesystems. "original" holds uploads
// exactly as they arrived. "processed" is a regenerable cache of transformed
// results, keyed by the FileSpec that produced them. "working" is a local
// directory of recently served files, evicted after a TTL.
//
// Serve walks those layers in reverse, generating each one only when it is
// missing. Non-media files are copied through unchanged, so FFmpeg is needed
// only for image, audio, and video transforms.
//
// Remote cover art is fetched through an SSRF-guarded client; see
// WithAllowedHosts and WithAllowPrivateIPs.
package mediaserver
