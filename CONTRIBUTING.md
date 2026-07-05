# Contributing to ffgo

Thanks for your interest in making FFmpeg friendlier for everyone. 🎬

## Getting started

```sh
git clone https://github.com/arbazkhan971/ffgo
cd ffgo
make build       # builds ./ffgo
./ffgo inspect testdata/sample.mp4
```

You'll need Go 1.22+ and FFmpeg installed and on your PATH.

## Development loop

```sh
make check       # fmt + vet + test
make test        # unit tests
make lint        # golangci-lint (optional locally, enforced in CI)
```

## Project layout

| Path                | Responsibility                                     |
| ------------------- | -------------------------------------------------- |
| `cmd/`              | One file per command; thin cobra wiring.           |
| `internal/ffmpeg`   | Execution engine: run, progress, dry-run.          |
| `internal/ffprobe`  | Typed media inspection.                            |
| `internal/ui`       | Colors, progress bars, tables, humanized values.   |
| `internal/presets`  | Compression profiles and size math.                |
| `internal/formats`  | Container registry and copy-vs-reencode logic.     |
| `internal/explain`  | FFmpeg argument dictionary.                         |
| `internal/ai`       | Natural-language providers.                         |

## Adding a command

1. Create `cmd/<name>.go`.
2. Build args, get the duration from a probe for the progress bar, and call
   `engine.Run`. Never shell out to ffmpeg directly — the engine handles
   progress, `--dry-run` and errors for you.
3. Register it in `func init()` with `rootCmd.AddCommand`.
4. Add a test and an example to the README.

## Guidelines

- Keep the CLI honest: every command must work with `--dry-run` and print the
  exact FFmpeg it would run.
- Excellent defaults, excellent errors. If a user could be confused, add a hint.
- Match the existing style. `gofmt`/`goimports` clean, small functions,
  documented exports.
- Conventional commit messages (`feat:`, `fix:`, `docs:` …) power the changelog.

## Reporting bugs

Open an issue with the command you ran, the output (add `--show-command`), and
your `ffgo version` output. A sample file or `ffgo inspect --json` helps a lot.
