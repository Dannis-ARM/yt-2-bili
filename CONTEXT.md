---
name: yt-2-bili-context
description: Core domain terminology for the yt-2-bili project
metadata:
  type: context
---

# Glossary

## Video
A piece of video content hosted on a platform.

## YouTube Video
A video hosted on YouTube. Identified by a URL. Has metadata including title, description, tags, etc.

## Bilibili Video
A video hosted on Bilibili. Has metadata including title, description, tags, etc.

## Download
The process of retrieving a YouTube Video's content and metadata from YouTube to local storage.

## Upload
The process of sending a video's content and metadata from local storage to Bilibili.

## Video Metadata
Information about a video including title, description, tags, and thumbnail.

## Bilibili Description
The description for a Bilibili Video, which includes the original YouTube video's description, plus the original author's name and YouTube URL.

## Playlist
A collection of YouTube videos. Not supported initially.

## yt-2-bili
A command-line tool written in Go that downloads YouTube Videos and uploads them to Bilibili.

## yt-dlp
An external CLI tool used for downloading YouTube videos and their metadata.

## biliup
An external CLI tool used for authenticating with Bilibili and uploading videos.
