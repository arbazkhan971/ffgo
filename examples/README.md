# ffgo cookbook

Real-world recipes. Every one prints the exact FFmpeg it runs — add `--dry-run`
to see the command without executing it.

## Shrink a screen recording for Slack

```sh
ffgo compress recording.mov --target 10mb
```

## Make a phone video WhatsApp-ready

```sh
ffgo compress IMG_4021.mov --preset whatsapp
```

## Turn a moment into a shareable GIF

```sh
ffgo gif gameplay.mp4 --from 1:30 --to 1:36 --width 600 --fps 20
```

## Cut the boring intro (losslessly)

```sh
ffgo trim webinar.mp4 --from 0:45 --to 58:20
```

## Grab just the audio as MP3

```sh
ffgo audio extract lecture.mkv --format mp3
```

## Even out a podcast's loudness

```sh
ffgo audio normalize episode.wav -o episode_mastered.wav
```

## Hardcode subtitles for social media

```sh
ffgo subtitles burn clip.mp4 --sub captions.srt
```

## Convert a whole folder of clips to MP4

```sh
ffgo batch "./footage/*.mov" --to mp4 -o ./mp4
```

## Compress every video under a size cap

```sh
ffgo batch "./exports/*.mp4" --compress --target 50mb -o ./small
```

## Understand a scary command from the internet

```sh
ffgo explain "ffmpeg -i in.mp4 -vf yadif,scale=1280:-2 -c:v libx264 -crf 20 out.mp4"
```

## Just tell it what you want

```sh
export FFGO_AI_PROVIDER=openai OPENAI_API_KEY=sk-...
ffgo ai "make a 15-second square clip starting at 0:30 for Instagram" reel.mp4
```

## Preview before committing

```sh
# See the FFmpeg command, change nothing:
ffgo compress big.mp4 --target 25mb --dry-run

# Run it, but print the command first:
ffgo compress big.mp4 --target 25mb --show-command
```
