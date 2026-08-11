---
name: video-platform
description: Handle LMS video metadata, secure delivery, storage and playback architecture.
---

# Video Platform

## Rule
Do not store video binary data in PostgreSQL.

Store only metadata:
- lesson_id
- video_url or storage key
- duration
- provider/status fields

## Architecture
Local development may use simple files.

Production target:
S3-compatible object storage -> CDN -> player

## Security
For paid/private content:
- do not expose unrestricted permanent object URLs if avoidable
- prefer signed/authorized delivery when the storage/CDN supports it
- backend decides whether the user may access the lesson

## Player
Track progress periodically without sending an excessive number of writes.

Persist:
- progress_seconds
- completed state
