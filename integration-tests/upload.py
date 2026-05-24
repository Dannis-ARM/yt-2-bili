from __future__ import annotations

from datetime import datetime
import sys

from helpers import BIN, DEFAULT_URL, build_cli, cookie_from_argv, require_file, require_tool, reset_dir, run, temp_case_dir


def main() -> None:
    cookie = cookie_from_argv(sys.argv)
    output_dir = temp_case_dir("yt-2-bili-integration-upload")
    video = output_dir / "upload-test.mp4"
    cover = output_dir / "cover.jpg"

    require_file(cookie, "Cookie file not found")
    require_tool("ffmpeg")
    reset_dir(output_dir)
    build_cli()

    print("Creating test video and cover...", flush=True)
    run([
        "ffmpeg",
        "-y",
        "-f", "lavfi",
        "-i", "color=c=black:s=1280x720:d=3",
        "-f", "lavfi",
        "-i", "anullsrc=channel_layout=stereo:sample_rate=44100",
        "-shortest",
        "-c:v", "libx264",
        "-pix_fmt", "yuv420p",
        "-c:a", "aac",
        str(video),
    ])
    run([
        "ffmpeg",
        "-y",
        "-f", "lavfi",
        "-i", "color=c=black:s=1280x720:d=1",
        "-frames:v", "1",
        "-update", "1",
        str(cover),
    ])

    title = "yt-2-bili integration upload " + datetime.now().strftime("%Y%m%d-%H%M%S")

    print("Running real Bilibili upload integration test...", flush=True)
    run([
        str(BIN),
        "upload",
        "--cookie", str(cookie),
        "--title", title,
        "--desc", "yt-2-bili integration upload test",
        "--cover", str(cover),
        "--source", DEFAULT_URL,
        "--tag", "test,yt-2-bili",
        "--tid", "171",
        str(video),
    ])

    print("Upload integration test passed.")
    print(f"Uploaded title: {title}")


if __name__ == "__main__":
    main()
