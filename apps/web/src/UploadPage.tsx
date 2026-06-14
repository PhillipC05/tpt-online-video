import { useState, type FormEvent } from 'react';

export default function UploadPage() {
  const [file, setFile] = useState<File | null>(null);
  const [uploadUrl, setUploadUrl] = useState<string | null>(null);
  const [videoId, setVideoId] = useState<string | null>(null);
  const [uploading, setUploading] = useState(false);
  const [progress, setProgress] = useState(0);
  const [error, setError] = useState<string | null>(null);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (!file) return;

    setError(null);
    setUploading(true);
    setProgress(0);

    try {
      // 1. Create upload session
      const sessionResp = await fetch('/api/v1/upload', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          filename: file.name,
          mime_type: file.type || 'application/octet-stream',
          byte_size: file.size,
        }),
      });

      if (!sessionResp.ok) {
        const err = await sessionResp.json();
        throw new Error(err.error || 'Failed to create upload session');
      }

      const session = await sessionResp.json();
      const chunkSize = 5 * 1024 * 1024; // 5MB chunks
      let offset = 0;

      // 2. Upload chunks
      while (offset < file.size) {
        const chunk = file.slice(offset, offset + chunkSize);
        const chunkResp = await fetch(`/api/v1/upload/${session.session_id}/chunk`, {
          method: 'POST',
          body: chunk,
        });

        if (!chunkResp.ok) {
          const err = await chunkResp.json();
          throw new Error(err.error || 'Chunk upload failed');
        }

        const chunkResult = await chunkResp.json();
        offset += chunk.size;
        setProgress(Math.round((offset / file.size) * 100));
      }

      // 3. Complete upload
      const completeResp = await fetch(`/api/v1/upload/${session.session_id}/complete`, {
        method: 'POST',
      });

      if (!completeResp.ok) {
        const err = await completeResp.json();
        throw new Error(err.error || 'Failed to complete upload');
      }

      const result = await completeResp.json();
      setVideoId(result.video_id);
      setUploadUrl(`/watch/${result.video_id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setUploading(false);
    }
  }

  if (videoId) {
    return (
      <main className="app-shell">
        <div className="card" style={{ textAlign: 'center', padding: '3rem' }}>
          <h2>Upload complete!</h2>
          <p>Your video is being transcoded.</p>
          <a href={uploadUrl!} className="button">
            Watch video
          </a>
          <br /><br />
          <a href="/upload" className="button" style={{ opacity: 0.7 }}>
            Upload another
          </a>
        </div>
      </main>
    );
  }

  return (
    <main className="app-shell">
      <div className="card" style={{ maxWidth: '600px', margin: '0 auto' }}>
        <h2>Upload a video</h2>
        <form onSubmit={handleSubmit} aria-busy={uploading}>
          <div className="form-field" style={{ marginBottom: '1rem' }}>
            <label htmlFor="video-file">Video file</label>
            <input
              id="video-file"
              type="file"
              accept="video/*"
              onChange={(e) => setFile(e.target.files?.[0] ?? null)}
              disabled={uploading}
              aria-describedby={error ? 'upload-error' : undefined}
            />
          </div>

          {error && (
            <p id="upload-error" role="alert" className="form-error" style={{ marginBottom: '1rem' }}>
              {error}
            </p>
          )}

          {uploading && (
            <div style={{ marginBottom: '1rem' }}>
              <progress
                value={progress}
                max={100}
                style={{ width: '100%' }}
                aria-label={`Upload progress: ${progress}%`}
              />
              <p style={{ fontSize: '0.875rem', textAlign: 'center' }} aria-live="polite">
                Uploading: {progress}%
              </p>
            </div>
          )}

          <button
            type="submit"
            disabled={!file || uploading}
            className="button"
            aria-busy={uploading}
          >
            {uploading ? 'Uploading…' : 'Upload'}
          </button>
        </form>
      </div>
    </main>
  );
}

export function UploadLink() {
  return (
    <a href="/upload" className="button" style={{ marginLeft: 'auto' }}>
      Upload
    </a>
  );
}