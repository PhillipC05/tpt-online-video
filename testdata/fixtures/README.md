# Test Fixtures

Minimal binary fixtures used by the test suite. These are **not** playable media files — they carry correct container magic bytes for file-type validation tests only.

| File | Format | Use |
|---|---|---|
| `sample.mp4` | MP4 (ftyp+mdat) | Upload/file-type validation tests |
| `sample.webm` | WebM (EBML header) | Upload/file-type validation tests |
| `not-a-video.pdf` | PDF | Rejection tests (non-video file) |

For integration tests that require actual decodable media (ffprobe/ffmpeg), generate fixtures with:

```sh
ffmpeg -f lavfi -i testsrc=duration=1:size=64x64:rate=1 \
       -f lavfi -i sine=frequency=440:duration=1 \
       -t 1 testdata/fixtures/playable.mp4
```
