from __future__ import annotations

import os
import sys

from helpers import BIN, DEFAULT_URL, build_cli, first_match, reset_dir, require_tool, run, temp_case_dir


def main() -> None:
    url = sys.argv[1] if len(sys.argv) > 1 else DEFAULT_URL
    output_dir = temp_case_dir("yt-2-bili-integration-subtitles")

    whisper_path = os.environ.get("YT_2_BILI_WHISPER_PATH", "whisper-ctranslate2")
    whisper_model_directory = os.environ.get("YT_2_BILI_WHISPER_MODEL_DIRECTORY", R"E:\Models\faster-whisper-large-v3")

    require_tool(whisper_path)
    require_tool("ffmpeg")
    require_tool("ffprobe")
    reset_dir(output_dir)
    build_cli()

    command = [
        str(BIN),
        "download",
        "--generate-subtitles",
        "--whisper-path",
        whisper_path,
        "--output-dir",
        str(output_dir),
    ]
    if whisper_model_directory:
        command.extend(["--whisper-model-directory", whisper_model_directory])
    command.append(url)

    print("Running subtitle integration test...", flush=True)
    run(command)

    video = first_match(output_dir, ["*.mp4"])
    subtitle = first_match(output_dir, ["*.srt"])
    subtitled_video = first_match(output_dir, ["*.subtitled.mp4"])

    print("Subtitle integration test passed.")
    print(f"Video: {video}")
    print(f"Subtitle: {subtitle}")
    print(f"Subtitled video: {subtitled_video}")


if __name__ == "__main__":
    main()
