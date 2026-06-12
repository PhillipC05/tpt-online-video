import { useCallback, useEffect, useState } from 'react';

// ─── Types ───────────────────────────────────────────────────────────────────

type CommentOwner = {
  id: string;
  display_name: string;
};

type CommentData = {
  id: string;
  video_id: string;
  user_id: string;
  parent_id: string | null;
  body: string;
  status: string;
  owner: CommentOwner | null;
  like_count: number;
  liked: boolean;
  replies?: CommentData[];
  created_at: string;
  updated_at: string;
};

// ─── Auth helpers ────────────────────────────────────────────────────────────

function getStoredToken(): string | null {
  try {
    return localStorage.getItem('access_token');
  } catch {
    return null;
  }
}

function authHeaders(): Record<string, string> {
  const token = getStoredToken();
  return token ? { Authorization: `Bearer ${token}` } : {};
}

// ─── Props ───────────────────────────────────────────────────────────────────

type CommentSectionProps = {
  videoId: string;
  currentUserId: string | null;
};

// ─── Main component ──────────────────────────────────────────────────────────

export default function CommentSection({ videoId, currentUserId }: CommentSectionProps) {
  const [comments, setComments] = useState<CommentData[]>([]);
  const [loading, setLoading] = useState(true);
  const [newBody, setNewBody] = useState('');
  const [sending, setSending] = useState(false);
  const [replyTo, setReplyTo] = useState<string | null>(null); // comment ID being replied to
  const [replyBodies, setReplyBodies] = useState<Record<string, string>>({});

  const fetchComments = useCallback(() => {
    fetch(`/api/v1/videos/${videoId}/comments`)
      .then((r) => (r.ok ? r.json() : []))
      .then((data: CommentData[]) => {
        setComments(data);
        setLoading(false);
      })
      .catch(() => setLoading(false));
  }, [videoId]);

  useEffect(() => {
    fetchComments();
  }, [fetchComments]);

  async function handleCreateComment(e: React.FormEvent) {
    e.preventDefault();
    const body = newBody.trim();
    if (!body) return;

    setSending(true);
    try {
      const res = await fetch(`/api/v1/videos/${videoId}/comments`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', ...authHeaders() },
        body: JSON.stringify({ body }),
      });
      if (res.ok) {
        setNewBody('');
        fetchComments();
      }
    } catch {
      // Silently fail
    } finally {
      setSending(false);
    }
  }

  async function handleCreateReply(parentId: string) {
    const body = (replyBodies[parentId] || '').trim();
    if (!body) return;

    setSending(true);
    try {
      const res = await fetch(`/api/v1/videos/${videoId}/comments`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', ...authHeaders() },
        body: JSON.stringify({ body, parent_id: parentId }),
      });
      if (res.ok) {
        setReplyTo(null);
        setReplyBodies((prev) => ({ ...prev, [parentId]: '' }));
        fetchComments();
      }
    } catch {
      // Silently fail
    } finally {
      setSending(false);
    }
  }

  async function handleDeleteComment(commentId: string) {
    if (!window.confirm('Delete this comment?')) return;
    try {
      const res = await fetch(`/api/v1/comments/${commentId}`, {
        method: 'DELETE',
        headers: authHeaders(),
      });
      if (res.ok || res.status === 204) {
        fetchComments();
      }
    } catch {
      // Silently fail
    }
  }

  async function handleLikeComment(commentId: string, liked: boolean) {
    try {
      const method = liked ? 'DELETE' : 'POST';
      await fetch(`/api/v1/comments/${commentId}/like`, {
        method,
        headers: authHeaders(),
      });
      fetchComments();
    } catch {
      // Silently fail
    }
  }

  async function handleReportComment(commentId: string) {
    const reason = window.prompt('Why are you reporting this comment?');
    if (!reason?.trim()) return;
    try {
      await fetch(`/api/v1/comments/${commentId}/report`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', ...authHeaders() },
        body: JSON.stringify({ reason: reason.trim() }),
      });
    } catch {
      // Silently fail
    }
  }

  function formatTime(iso: string): string {
    const d = new Date(iso);
    const now = new Date();
    const diffMs = now.getTime() - d.getTime();
    const diffMins = Math.floor(diffMs / 60000);
    if (diffMins < 1) return 'just now';
    if (diffMins < 60) return `${diffMins}m ago`;
    const diffHrs = Math.floor(diffMins / 60);
    if (diffHrs < 24) return `${diffHrs}h ago`;
    const diffDays = Math.floor(diffHrs / 24);
    if (diffDays < 7) return `${diffDays}d ago`;
    return d.toLocaleDateString();
  }

  // ── Render ─────────────────────────────────────────────────────────────────

  return (
    <section className="comment-section">
      <h3 className="comment-section-title">
        Comments ({comments.length})
      </h3>

      {/* New comment form */}
      {currentUserId ? (
        <form className="comment-form" onSubmit={handleCreateComment}>
          <textarea
            className="comment-input"
            placeholder="Add a comment…"
            value={newBody}
            onChange={(e) => setNewBody(e.target.value)}
            rows={3}
            maxLength={5000}
          />
          <div className="comment-form-actions">
            <span className="comment-char-count">{newBody.length}/5000</span>
            <button type="submit" className="comment-btn" disabled={sending || !newBody.trim()}>
              {sending ? 'Posting…' : 'Comment'}
            </button>
          </div>
        </form>
      ) : (
        <p className="comment-login-prompt">
          <a href="/auth/login" style={{ color: '#60a5fa' }}>Log in</a> to leave a comment.
        </p>
      )}

      {/* Comment list */}
      {loading ? (
        <p className="comment-loading">Loading comments…</p>
      ) : comments.length === 0 ? (
        <p className="comment-empty">No comments yet. Be the first!</p>
      ) : (
        <div className="comment-list">
          {comments.map((comment) => (
            <CommentCard
              key={comment.id}
              comment={comment}
              currentUserId={currentUserId}
              replyTo={replyTo}
              replyBodies={replyBodies}
              sending={sending}
              onReplyToggle={(id) => setReplyTo(replyTo === id ? null : id)}
              onReplyBodyChange={(id, body) => setReplyBodies((prev) => ({ ...prev, [id]: body }))}
              onSubmitReply={handleCreateReply}
              onDelete={handleDeleteComment}
              onLike={handleLikeComment}
              onReport={handleReportComment}
              formatTime={formatTime}
              depth={0}
            />
          ))}
        </div>
      )}
    </section>
  );
}

// ─── Comment card ────────────────────────────────────────────────────────────

type CommentCardProps = {
  comment: CommentData;
  currentUserId: string | null;
  replyTo: string | null;
  replyBodies: Record<string, string>;
  sending: boolean;
  depth: number;
  onReplyToggle: (id: string) => void;
  onReplyBodyChange: (id: string, body: string) => void;
  onSubmitReply: (parentId: string) => Promise<void>;
  onDelete: (id: string) => void;
  onLike: (id: string, liked: boolean) => void;
  onReport: (id: string) => void;
  formatTime: (iso: string) => string;
};

function CommentCard({
  comment,
  currentUserId,
  replyTo,
  replyBodies,
  sending,
  depth,
  onReplyToggle,
  onReplyBodyChange,
  onSubmitReply,
  onDelete,
  onLike,
  onReport,
  formatTime,
}: CommentCardProps) {
  const isOwner = currentUserId !== null && currentUserId === comment.user_id;
  const isHidden = comment.status === 'hidden' && !isOwner;

  return (
    <div className={`comment-card${depth > 0 ? ' comment-reply' : ''}`} style={{ marginLeft: depth > 0 ? `${Math.min(depth, 4) * 32}px` : 0 }}>
      <div className="comment-card-header">
        <span className="comment-author">
          {comment.owner?.display_name ?? 'Unknown'}
        </span>
        <span className="comment-time">{formatTime(comment.created_at)}</span>
      </div>

      <div className="comment-body">
        {isHidden ? (
          <em style={{ color: '#6b7280' }}>[comment hidden]</em>
        ) : (
          comment.body
        )}
      </div>

      <div className="comment-actions">
        {/* Like button */}
        <button
          className={`comment-action-btn${comment.liked ? ' comment-action-btn--active' : ''}`}
          onClick={() => onLike(comment.id, comment.liked)}
          disabled={!currentUserId}
          title={currentUserId ? (comment.liked ? 'Unlike' : 'Like') : 'Log in to like'}
        >
          {comment.liked ? '👍' : '👍'} {comment.liked && comment.like_count > 1 ? '' : ''}
          {comment.like_count > 0 && <span className="comment-like-count">{comment.like_count}</span>}
        </button>

        {/* Reply button */}
        {currentUserId && !isHidden && (
          <button
            className="comment-action-btn"
            onClick={() => onReplyToggle(comment.id)}
          >
            {replyTo === comment.id ? 'Cancel' : 'Reply'}
          </button>
        )}

        {/* Edit/Delete for owner */}
        {isOwner && !isHidden && (
          <button
            className="comment-action-btn comment-action-btn--danger"
            onClick={() => onDelete(comment.id)}
          >
            Delete
          </button>
        )}

        {/* Report for non-owners */}
        {currentUserId && !isOwner && (
          <button
            className="comment-action-btn comment-action-btn--danger"
            onClick={() => onReport(comment.id)}
          >
            Report
          </button>
        )}
      </div>

      {/* Reply form */}
      {replyTo === comment.id && (
        <div className="comment-reply-form">
          <textarea
            className="comment-input"
            placeholder="Write a reply…"
            value={replyBodies[comment.id] || ''}
            onChange={(e) => onReplyBodyChange(comment.id, e.target.value)}
            rows={2}
            maxLength={5000}
          />
          <div className="comment-form-actions">
            <span className="comment-char-count">{(replyBodies[comment.id] || '').length}/5000</span>
            <button
              className="comment-btn"
              onClick={() => onSubmitReply(comment.id)}
              disabled={sending || !(replyBodies[comment.id] || '').trim()}
            >
              {sending ? 'Posting…' : 'Reply'}
            </button>
          </div>
        </div>
      )}

      {/* Nested replies */}
      {comment.replies && comment.replies.length > 0 && (
        <div className="comment-replies">
          {comment.replies.map((reply) => (
            <CommentCard
              key={reply.id}
              comment={reply}
              currentUserId={currentUserId}
              replyTo={replyTo}
              replyBodies={replyBodies}
              sending={sending}
              depth={depth + 1}
              onReplyToggle={onReplyToggle}
              onReplyBodyChange={onReplyBodyChange}
              onSubmitReply={onSubmitReply}
              onDelete={onDelete}
              onLike={onLike}
              onReport={onReport}
              formatTime={formatTime}
            />
          ))}
        </div>
      )}
    </div>
  );
}