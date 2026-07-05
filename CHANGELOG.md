# Changelog

All notable changes to ffgo are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-07-05

First public release.

### Added
- `inspect` — readable media summary (codecs, resolution, bitrate, tracks,
  HDR, metadata) with smart recommendations, plus `--json` for raw ffprobe.
- `convert` — container conversion with automatic stream-copy when possible.
- `compress` — target-size, quality-level and platform presets
  (whatsapp, youtube, discord, telegram, twitter, web, email).
- `trim` — lossless (keyframe) and frame-accurate re-encode cutting.
- `gif` — high-quality GIFs via a generated palette.
- `audio` — extract, normalize (EBU R128), silence-remove, convert.
- `subtitles` — burn, extract, convert.
- `batch` — glob-driven bulk convert/compress.
- `explain` — plain-English breakdown of any FFmpeg command.
- `ai` — natural-language to FFmpeg, with OpenAI / Anthropic / Gemini /
  Ollama / OpenRouter and any OpenAI-compatible endpoint.
- Global `--dry-run`, `--show-command`, `-y`, `-q`, `--color`.
- Live progress bars, colorized output, and a zero-dependency UI layer.

[Unreleased]: https://github.com/arbazkhan971/ffgo/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/arbazkhan971/ffgo/releases/tag/v0.1.0
