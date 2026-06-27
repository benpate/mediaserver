# Media Server 🌇

[![GoDoc](https://img.shields.io/badge/go-documentation-blue.svg?style=flat-square)](http://pkg.go.dev/github.com/benpate/mediaserver)
[![Version](https://img.shields.io/github/v/release/benpate/mediaserver?include_prereleases&style=flat-square&color=brightgreen)](https://github.com/benpate/mediaserver/releases)
[![Build Status](https://img.shields.io/github/actions/workflow/status/benpate/mediaserver/go.yml?style=flat-square)](https://github.com/benpate/mediaserver/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/benpate/mediaserver?style=flat-square)](https://goreportcard.com/report/github.com/benpate/mediaserver)
[![Codecov](https://img.shields.io/codecov/c/github/benpate/mediaserver.svg?style=flat-square)](https://codecov.io/gh/benpate/mediaserver)


Media Server is a media manipulation library that works inside of your existing applications.  It manages uploads and downloads, and lets you translate files into different encodings on the fly using FFmpeg.  It works primarily with images and audio files, with video manipulations currently under development.

```go

// originals, processed are afero.Fs filesystems; working is a local temp area.
ms := mediaserver.New(originals, processed, working)

// Add an original file
if err := ms.Put("myfile.png", fileData); err != nil {
  // handle error
}

// Describe the processed file you want (resized, transcoded, etc.)
filespec := mediaserver.FileSpec{
  Filename:          "myfile.png",
  OriginalExtension: ".png",
  Extension:         ".webp",
  Width:             600,
  Height:            600,
}

// Serve writes the (cached or freshly processed) file to the HTTP response
if err := ms.Serve(responseWriter, request, filespec); err != nil {
  // handle error
}
```

Use [`Process`](https://pkg.go.dev/github.com/benpate/mediaserver#MediaServer.Process) instead of `Serve` to write the result to any `io.Writer`, and [`ServeOriginal`](https://pkg.go.dev/github.com/benpate/mediaserver#MediaServer.ServeOriginal) to return an unmodified upload.

## Image Resizing

Media Server can resize and transcode images. Just request an image with a [FileSpec](https://pkg.go.dev/github.com/benpate/mediaserver#FileSpec) that matches your needs and the corresponding file will be generated (or retrieved from the cache) and returned to your calling application.

```go
// This filespec resizes an image to 1200px (maintaining aspect ratio)
filespec := mediaserver.Filespec{
  Width:1200,
}
```

## Media Transcoding

Media Server can automatically translate files between these formats:

```go
// This filespec converts an audio file into an MP3
filespec := mediaserver.Filespec{
  MimeType:"audio/mp3"
}
```

**Image Types**: GIF, JPG, PNG, WEBP

**Audio Types**: AAC, FLAC, M4A, MP3, OGG, OPUS

**Video Types**: Coming soon

## FFmpeg Dependency

This library now depends on [FFmpeg](https://ffmpeg.org) for all media manipulations.  This eliminated a problematic dependency on CGo, and has expanded the kinds of media files that mediaserver can manipulate.

Media Server maintains two resource directories: one that contains original uploads and a cache of modified or transcoded files.

## Afero Filesystems

Media server uses [Afero](https://github.com/spf13/afero) to connect to both of the file directories (one for originals, and one for cached results).  Afero is a filesystem abstraction with connectors for many different kinds of directory services, including: 

* local and networked directories
* memory disks
* [S3](https://github.com/fclairamb/afero-s3)
* [HTTP](https://github.com/spf13/afero/blob/master/httpFs.go)
* [Git](https://github.com/go-git/go-git)
* [Dropbox](https://github.com/fclairamb/afero-dropbox)
* [Google Cloud Storage](https://github.com/spf13/afero/tree/master/gcsfs)
* [SFTP](https://github.com/spf13/afero/tree/master/sftpfs)

## What matters here

- **Three filesystems, three roles.** `original` holds untouched uploads, `processed` is a regenerable cache of transcoded results (safe to wipe), and `working` is a local temp area with a TTL. `Serve` walks original → processed → working on demand, generating each layer only when it is missing.

- **Cover-image fetching is SSRF-hardened.** `FileSpec.Metadata["cover"]` is an arbitrary URL that the server downloads. It goes through an SSRF-guarded [remote](https://github.com/benpate/remote) client: only `http`/`https` schemes are allowed, private/loopback IPs are blocked by default (opt in with `WithAllowPrivateIPs`), an optional host allow-list is enforced (`WithAllowedHosts`), and the body is size-capped. FFmpeg then reads only the local file, with `-protocol_whitelist file` so a disguised playlist can't reach back out.

- **A missing or blocked cover is not fatal.** If the cover download fails, it is logged and the media is processed without art — it never aborts the request.

- **The `working` directory must be `Close`d.** `NewWorkingDirectory` launches a background eviction goroutine; failing to `Close` it leaks the goroutine and leaves temp files behind.

- **FFmpeg is required for media transforms.** Non-media files are copied through verbatim, but image/audio/video processing fails cleanly if `ffmpeg` is not on the `PATH`.

## Pull Requests Welcome

This library is a work in progress, and will benefit from your experience reports, use cases, and contributions.  If you have an idea for making Media Server better, send in a pull request.  We're all in this together! 🌇
