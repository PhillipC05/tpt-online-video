# Broadcaster Guide

This guide explains how to upload videos, go live, and manage your channel on TPT Online Video.

---

## Creating an account

Navigate to the platform URL and click **Sign Up**. Enter your email, a display name, and a password. After registering, you are logged in immediately.

---

## Your channel

Your channel page is available at `/channels/{your-user-id}`. It shows your public videos and live streams. You can customise it:

1. Go to **Profile Settings** (top-right menu → Settings).
2. Upload a **profile picture** (JPEG or PNG, max 5 MB).
3. Upload a **banner image** (JPEG or PNG, max 5 MB, recommended 1920×480 px).
4. Edit your **display name** and **bio**.
5. Click **Save**.

---

## Uploading a video

1. Click **Upload** in the navigation bar.
2. Select your video file (MP4, MOV, MKV, and other common formats are accepted).
3. While the file uploads, fill in:
   - **Title** (required, max 200 characters)
   - **Description** (optional, max 5 000 characters)
   - **Visibility** — `Public`, `Unlisted` (link-only), or `Private`
4. Click **Publish** once the upload completes.

The video enters the transcoding pipeline and becomes available for playback once processing finishes (status: `Ready`). Transcoding generates multiple quality levels: 1080p, 720p, 480p, and 360p.

### Upload tips

- Uploads resume automatically if your connection drops mid-transfer. Do not close the browser tab until you see the upload completed message.
- For very large files (multi-GB), use a wired connection if possible.
- Maximum upload size is set by the platform administrator.

### Editing a video after upload

Go to your channel → click the video → **Edit** (pencil icon). You can change the title, description, and visibility at any time.

### Deleting a video

Channel page → video → **Edit** → **Delete**. Deletion is soft — the video is removed from public view but may be recoverable by an admin.

---

## Going live

### One-time setup

1. Go to **Live** in the navigation bar.
2. Click **Create Stream**.
3. Enter a **title**, optional **description**, and **visibility** setting.
4. Copy the **Stream Key** — it is shown **only once**. Store it in your streaming software now.
5. Note the **RTMP URL** (e.g. `rtmp://your-domain.com/live`).

> If you lose your stream key, delete the stream and create a new one.

### Streaming with OBS Studio

1. Open OBS → **Settings** → **Stream**.
2. Set **Service** to `Custom`.
3. Paste the **Server** URL (`rtmp://your-domain.com/live`).
4. Paste the **Stream Key**.
5. Click **OK**, then **Start Streaming**.

Your stream goes live within a few seconds. Viewers can watch via HLS (all browsers) or WebRTC (low-latency, ~1 second delay).

### Recommended OBS settings

| Setting | Recommended value |
|---------|------------------|
| Encoder | x264 or hardware (NVENC/AMF) |
| Bitrate | 4 000–8 000 Kbps for 1080p |
| Keyframe interval | 2 seconds |
| Audio bitrate | 160 Kbps |
| Resolution | 1920×1080 |
| Frame rate | 30 or 60 fps |

### During a live stream

- Your stream appears on the **Live** page and your channel while broadcasting.
- **Live chat** is available to viewers. You can moderate it directly from the watch page (see [Chat moderation](#chat-moderation) below).
- DVR allows viewers to seek back up to 15 minutes in the current stream.

### Ending a stream

Stop streaming in OBS. The stream status changes to `ended` within a few seconds.

---

## Chat moderation

As the stream owner you can moderate your own live chat from the watch page without needing a moderator role.

### Available actions

| Action | How |
|--------|-----|
| Delete a message | Hover the message → trash icon |
| Timeout a user | Click username → Timeout → choose duration |
| Ban a user from your chat | Click username → Ban |
| Lock chat (no new messages) | Chat settings → Lock Chat |
| Unlock chat | Chat settings → Unlock Chat |

Bans and timeouts apply to your stream. Platform-wide bans require an admin or moderator.

---

## Managing streams

Go to **Live** in the navigation bar to see all your streams (past and present).

- **Edit** — change title, description, or visibility before or during a stream.
- **Delete** — remove the stream listing. Deleting while live terminates the broadcast.

---

## Visibility settings

| Setting | Who can see it |
|---------|---------------|
| `Public` | Everyone, listed on the platform |
| `Unlisted` | Only people with the direct link |
| `Private` | Only you |

---

## Getting help

If a video is stuck in `Processing` for more than 30 minutes, contact your platform administrator. Transcoding errors are visible to admins in the system status panel.
