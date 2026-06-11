# System Context Diagram

```mermaid
C4Context
  Person(user, "User", "Viewer or broadcaster")
  Person(admin, "Admin", "Platform administrator or moderator")
  System(tpt, "TPT Online Video", "Self-hostable video platform")

  System_Ext(obs, "OBS Studio", "Live streaming encoder")
  System_Ext(browser, "Web Browser", "Chrome, Firefox, Safari, Edge")
  System_Ext(s3, "S3-compatible Storage", "MinIO, AWS S3, Wasabi")
  System_Ext(email, "SMTP Server", "Optional email for password resets")
  System_Ext(oauth, "OAuth Providers", "Google, GitHub")

  Rel(user, tpt, "Watches VOD/live, uploads, searches, comments")
  Rel(admin, tpt, "Moderates reports, manages users, views audit log")
  Rel(obs, tpt, "RTMP ingest")
  Rel(browser, tpt, "HTTP, WebRTC, WebSocket")
  Rel(tpt, s3, "Stores media objects")
  Rel(tpt, email, "Sends transactional emails")
  Rel(tpt, oauth, "Authenticates via OAuth")