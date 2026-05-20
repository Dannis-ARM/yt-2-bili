# 1. Use external tools yt-dlp and biliup

Date: 2026-05-21

## Status

Accepted

## Context

We need to build a tool that downloads YouTube videos and uploads them to Bilibili.

Implementing YouTube download and Bilibili upload from scratch would be:
- Time-consuming
- Prone to breaking as platforms change their APIs
- Requiring ongoing maintenance

## Decision

We will use two existing external tools:
1. **yt-dlp** for downloading YouTube videos and metadata
2. **biliup-rs** for uploading to Bilibili

Our Go program will act as a coordinator:
- Call `yt-dlp` to download video, thumbnail, and metadata
- Construct Bilibili description with original author and YouTube link
- Call `biliup upload` with the appropriate parameters

## Consequences

### Positive
- Leverage mature, well-maintained tools
- Faster development
- Less maintenance burden when platforms change
- Benefit from features and fixes in yt-dlp and biliup-rs

### Negative
- Users need to install yt-dlp and biliup-rs separately
- We depend on the CLI interfaces of these tools remaining stable

## Implementation Details

### yt-dlp usage
1. First call `yt-dlp -J <url>` to get metadata as JSON
2. Then call `yt-dlp` with:
   - `-f 'bestvideo[height<=1080]+bestaudio/best[height<=1080]'` for 1080p
   - `--write-thumbnail` to download thumbnail
   - `-o` to specify output path

### biliup-rs usage
Call `biliup upload` with:
- `--title` from YouTube
- `--desc` constructed with original description, author, and YouTube link
- `--tag` from YouTube tags
- `--cover` path to downloaded thumbnail
- `--copyright 2` (reupload)
- `--source` YouTube URL
- `--tid` user-specified or default 171
