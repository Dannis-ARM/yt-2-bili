from __future__ import annotations

import sys

from helpers import BIN, DEFAULT_URL, build_cli, cookie_from_argv, first_match, require_file, reset_dir, run, temp_case_dir


def main() -> None:
    cookie = cookie_from_argv(sys.argv)
    url = sys.argv[2] if len(sys.argv) > 2 else DEFAULT_URL
    output_dir = temp_case_dir("yt-2-bili-integration-e2e")

    require_file(cookie, "Cookie file not found")
    reset_dir(output_dir)
    build_cli()

    print("Running real end-to-end integration test...", flush=True)
    run([
        str(BIN),
        "transfer",
        "--cookie", str(cookie),
        "--keep-video",
        "--output-dir", str(output_dir),
        url,
    ])

    video = first_match(output_dir, ["*.mp4"])

    print("End-to-end integration test passed.")
    print(f"Kept downloaded video: {video}")


if __name__ == "__main__":
    main()
