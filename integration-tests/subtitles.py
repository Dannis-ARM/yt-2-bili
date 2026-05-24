from __future__ import annotations

import sys

from helpers import BIN, DEFAULT_URL, build_cli, first_match, reset_dir, require_tool, run, temp_case_dir


def main() -> None:
    url = sys.argv[1] if len(sys.argv) > 1 else DEFAULT_URL
    output_dir = temp_case_dir("yt-2-bili-integration-subtitles")

    require_tool("whisper")
    require_tool("ffmpeg")
    require_tool("ffprobe")
    reset_dir(output_dir)
    build_cli()

    print("Running subtitle integration test...", flush=True)
    run([str(BIN), "download", "--generate-subtitles", "--output-dir", str(output_dir), url])

    video = first_match(output_dir, ["*.mp4"])
    subtitle = first_match(output_dir, ["*.srt"])
    subtitled_video = first_match(output_dir, ["*.subtitled.mp4"])

    print("Subtitle integration test passed.")
    print(f"Video: {video}")
    print(f"Subtitle: {subtitle}")
    print(f"Subtitled video: {subtitled_video}")


if __name__ == "__main__":
    main()
