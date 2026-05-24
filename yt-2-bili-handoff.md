# yt-2-bili Handoff Document

Date: 2026-05-21

## Current Status

A complete working prototype has been implemented. The tool is functional but hasn't been tested with actual video downloads and uploads yet.

## What We Built

yt-2-bili is a Go command-line tool that:
- Downloads YouTube videos using `yt-dlp`
- Uploads to Bilibili using `biliup`
- Handles metadata (title, description, tags, thumbnail)
- Adds attribution (original author + YouTube link) to Bilibili description

## Project Structure

See the repository at `e:\Projects\GoProject\yt-2-bili`:
- `cmd/yt-2-bili/main.go` - CLI interface using Cobra
- `internal/ytdlp/` - yt-dlp integration
- `internal/biliup/` - biliup integration
- `internal/workflow/` - coordinates the full process
- `docs/adr/0001-use-external-tools.md` - Architecture Decision Record
- `CONTEXT.md` - Domain glossary

## Suggested Next Steps

1. **Testing** - Test the tool with an actual YouTube video
2. **Error Handling** - Add more robust error handling and retries
3. **Playlist Support** - Implement playlist downloading (currently errors out)
4. **Additional Features** - Consider adding more yt-dlp/biliup options

## Suggested Skills

- `/tdd` - If adding new features with tests
- `/simplify` - If refactoring code
- `/review` - If creating a pull request
