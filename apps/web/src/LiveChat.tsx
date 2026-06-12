import { useCallback, useEffect, useRef, useState } from 'react';

// ─── Types ────────────────────────────────────────────────────────────────────

interface ChatMessage {
  id: string;
  stream_id: string;
  user_id?: string;
  display_name: string;
  body: string;
  deleted: boolean;
  created_at: string;
}

type ConnStatus = 'connecting' | 'connected' | 'disconnected';

interface Props {
  streamId: string;
  currentUserId?: string;
  currentDisplayName?: string;
  isOwner?: boolean;
  onDeleteMessage?: (messageId: string) => void;
}

// ─── Auth helper ──────────────────────────────────────────────────────────────

function getToken(): string | null {
  try {
    return localStorage.getItem('token');
  } catch {
    return null;
  }
}

// ─── Component ───────────────────────────────────────────────────────────────

export default function LiveChat({
  streamId,
  currentUserId,
  currentDisplayName,
  isOwner = false,
}: Props) {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [status, setStatus] = useState<ConnStatus>('connecting');
  const [chatLocked, setChatLocked] = useState(false);
  const [bannedOrTimedOut, setBannedOrTimedOut] = useState<string | null>(null);
  const [input, setInput] = useState('');
  const [sending, setSending] = useState(false);

  const wsRef = useRef<WebSocket | null>(null);
  const messagesEndRef = useRef<HTMLDivElement | null>(null);
  const reconnectTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const isMounted = useRef(true);

  const scrollToBottom = useCallback(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, []);

  const addSystemMessage = useCallback((text: string) => {
    const sys: ChatMessage = {
      id: `sys-${Date.now()}`,
      stream_id: streamId,
      display_name: 'System',
      body: text,
      deleted: false,
      created_at: new Date().toISOString(),
    };
    setMessages(prev => [...prev, sys]);
  }, [streamId]);

  const connect = useCallback(() => {
    if (!isMounted.current) return;

    setStatus('connecting');

    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const token = getToken();
    const tokenParam = token ? `?token=${encodeURIComponent(token)}` : '';
    const url = `${proto}//${window.location.host}/api/v1/live/streams/${streamId}/chat/ws${tokenParam}`;

    const ws = new WebSocket(url);
    wsRef.current = ws;

    ws.onopen = () => {
      if (!isMounted.current) return;
      setStatus('connected');
    };

    ws.onclose = () => {
      if (!isMounted.current) return;
      setStatus('disconnected');
      // Reconnect after 3 s
      reconnectTimer.current = setTimeout(() => {
        if (isMounted.current) connect();
      }, 3000);
    };

    ws.onerror = () => {
      ws.close();
    };

    ws.onmessage = (event: MessageEvent) => {
      if (!isMounted.current) return;
      let envelope: { type: string; data?: unknown };
      try {
        envelope = JSON.parse(event.data as string);
      } catch {
        return;
      }

      switch (envelope.type) {
        case 'history': {
          const { messages: hist } = envelope.data as { messages: ChatMessage[] };
          setMessages([...(hist ?? [])].reverse());
          break;
        }
        case 'message': {
          const msg = envelope.data as ChatMessage;
          setMessages(prev => [...prev, msg]);
          setTimeout(scrollToBottom, 50);
          break;
        }
        case 'deleted': {
          const { message_id } = envelope.data as { message_id: string };
          setMessages(prev =>
            prev.map(m => (m.id === message_id ? { ...m, deleted: true, body: '' } : m))
          );
          break;
        }
        case 'chat_locked':
          setChatLocked(true);
          addSystemMessage('Chat has been locked by the streamer.');
          break;
        case 'chat_unlocked':
          setChatLocked(false);
          addSystemMessage('Chat has been unlocked.');
          break;
        case 'banned':
          setBannedOrTimedOut('You have been banned from this chat.');
          addSystemMessage('You have been banned from this chat.');
          break;
        case 'timed_out': {
          const { expires_at } = envelope.data as { expires_at: string };
          const until = new Date(expires_at).toLocaleTimeString();
          const msg = `You are timed out until ${until}.`;
          setBannedOrTimedOut(msg);
          addSystemMessage(msg);
          break;
        }
        case 'timeout_removed':
          setBannedOrTimedOut(null);
          addSystemMessage('Your timeout has been lifted.');
          break;
        case 'joined':
          // acknowledged connection
          break;
        case 'error': {
          const { message: errMsg } = envelope.data as { message: string };
          addSystemMessage(`Error: ${errMsg}`);
          break;
        }
        default:
          break;
      }
    };
  }, [streamId, scrollToBottom, addSystemMessage]);

  useEffect(() => {
    isMounted.current = true;
    connect();

    return () => {
      isMounted.current = false;
      if (reconnectTimer.current) clearTimeout(reconnectTimer.current);
      wsRef.current?.close();
    };
  }, [connect]);

  const sendMessage = useCallback(() => {
    const body = input.trim();
    if (!body || !currentUserId || status !== 'connected' || sending) return;

    setSending(true);
    try {
      wsRef.current?.send(JSON.stringify({ type: 'message', body }));
      setInput('');
    } finally {
      setSending(false);
    }
  }, [input, currentUserId, status, sending]);

  const handleKeyDown = useCallback((e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      sendMessage();
    }
  }, [sendMessage]);

  // ─── Render ──────────────────────────────────────────────────────────────

  const canSend =
    !!currentUserId &&
    status === 'connected' &&
    !chatLocked &&
    !bannedOrTimedOut &&
    !sending;

  const dotClass =
    status === 'connected'
      ? 'live-chat-dot'
      : status === 'connecting'
      ? 'live-chat-dot live-chat-dot--connecting'
      : 'live-chat-dot live-chat-dot--disconnected';

  const statusLabel =
    status === 'connected' ? 'Live' : status === 'connecting' ? 'Connecting…' : 'Disconnected';

  return (
    <div className="live-chat">
      {/* Header */}
      <div className="live-chat-header">
        <span>Live Chat</span>
        <span className="live-chat-status">
          <span className={dotClass} />
          {statusLabel}
        </span>
      </div>

      {/* Messages */}
      <div className="live-chat-messages">
        {messages.map(msg => {
          if (msg.display_name === 'System') {
            return (
              <div key={msg.id} className="live-chat-msg live-chat-msg--system">
                {msg.body}
              </div>
            );
          }
          const isSelf = msg.user_id === currentUserId;
          return (
            <div
              key={msg.id}
              className={`live-chat-msg${msg.deleted ? ' live-chat-msg--deleted' : ''}`}
            >
              <span className={`live-chat-msg-author${isSelf ? ' live-chat-msg-author--self' : ''}`}>
                {msg.display_name}:
              </span>
              <span className="live-chat-msg-body">
                {msg.deleted ? '[deleted]' : msg.body}
              </span>
              {isOwner && !msg.deleted && (
                <DeleteButton messageId={msg.id} streamId={streamId} />
              )}
            </div>
          );
        })}
        <div ref={messagesEndRef} />
      </div>

      {/* Locked notice */}
      {chatLocked && (
        <div className="live-chat-locked-notice">Chat is locked</div>
      )}

      {/* Banned/timed out notice */}
      {bannedOrTimedOut && (
        <div className="live-chat-locked-notice" style={{ color: '#f87171' }}>
          {bannedOrTimedOut}
        </div>
      )}

      {/* Input */}
      {currentUserId ? (
        <div className="live-chat-form">
          <input
            className="live-chat-input"
            type="text"
            placeholder={canSend ? `Chat as ${currentDisplayName ?? 'you'}…` : 'Chat unavailable'}
            value={input}
            disabled={!canSend}
            maxLength={500}
            onChange={e => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
          />
          <button className="live-chat-send" disabled={!canSend || !input.trim()} onClick={sendMessage}>
            Send
          </button>
        </div>
      ) : (
        <div className="live-chat-anon-notice">
          <a href="/login">Sign in</a> to chat
        </div>
      )}
    </div>
  );
}

// ─── Delete button (owner only) ───────────────────────────────────────────────

function DeleteButton({ messageId, streamId }: { messageId: string; streamId: string }) {
  const [deleting, setDeleting] = useState(false);

  const handleDelete = async () => {
    if (deleting) return;
    setDeleting(true);
    try {
      const token = getToken();
      await fetch(`/api/v1/live/streams/${streamId}/chat/messages/${messageId}`, {
        method: 'DELETE',
        headers: token ? { Authorization: `Bearer ${token}` } : {},
      });
    } finally {
      setDeleting(false);
    }
  };

  return (
    <button
      onClick={handleDelete}
      disabled={deleting}
      title="Delete message"
      style={{
        marginLeft: '6px',
        background: 'none',
        border: 'none',
        color: '#64748b',
        cursor: 'pointer',
        fontSize: '0.7rem',
        padding: '0 2px',
        lineHeight: 1,
        verticalAlign: 'middle',
      }}
    >
      ×
    </button>
  );
}
