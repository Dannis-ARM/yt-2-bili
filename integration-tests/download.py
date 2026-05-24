from __future__ import annotations

import sys

from helpers import BIN, DEFAULT_URL, build_cli, first_match, reset_dir, run, temp_case_dir


def main() -> None:
    url = sys.argv[1] if len(sys.argv) > 1 else DEFAULT_URL
    output_dir = temp_case_dir("yt-2-bili-integration-download")

    reset_dir(output_dir)
    build_cli()

    print("Running download integration test...", flush=True)
    run([str(BIN), "download", "--output-dir", str(output_dir), url])

    video = first_match(output_dir, ["*.mp4"])
    thumbnail = first_match(output_dir, ["*.webp", "*.jpg", "*.jpeg", "*.png"])

    print("Download integration test passed.")
    print(f"Video: {video}")
    print(f"Thumbnail: {thumbnail}")


if __name__ == "__main__":
    main()
