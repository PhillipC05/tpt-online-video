import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, act, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import LiveChat from '../LiveChat';

// Minimal WebSocket mock
class MockWebSocket {
  static CONNECTING = 0;
  static OPEN = 1;
  static CLOSING = 2;
  static CLOSED = 3;

  readyState = MockWebSocket.OPEN;
  onopen: (() => void) | null = null;
  onclose: ((e: { code: number; reason: string }) => void) | null = null;
  onmessage: ((e: { data: string }) => void) | null = null;
  onerror: ((e: Event) => void) | null = null;

  sent: string[] = [];

  constructor(public url: string) {
    MockWebSocket.instances.push(this);
    // Simulate successful open on next tick
    setTimeout(() => this.onopen?.(), 0);
  }

  send(data: string) {
    this.sent.push(data);
  }

  close() {
    this.readyState = MockWebSocket.CLOSED;
  }

  // Deliver an envelope { type, data } from the server
  receive(envelope: object) {
    this.onmessage?.({ data: JSON.stringify(envelope) });
  }

  static instances: MockWebSocket[] = [];
  static reset() {
    MockWebSocket.instances = [];
  }
}

beforeEach(() => {
  MockWebSocket.reset();
  vi.stubGlobal('WebSocket', MockWebSocket);
  localStorage.clear();
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

async function waitForOpen() {
  await act(async () => {
    await new Promise((r) => setTimeout(r, 10));
  });
}

describe('LiveChat', () => {
  it('renders without crashing', () => {
    render(<LiveChat streamId="stream-1" />);
    expect(document.body).toBeInTheDocument();
  });

  it('shows connecting status initially', () => {
    render(<LiveChat streamId="stream-1" />);
    // Before the WebSocket open fires, status should be "connecting"
    expect(screen.getByText(/connecting/i)).toBeInTheDocument();
  });

  it('shows Live status after WebSocket opens', async () => {
    render(<LiveChat streamId="stream-1" />);
    await waitForOpen();

    // The status element has class "live-chat-status" and shows "Live"
    const statusEl = document.querySelector('.live-chat-status');
    expect(statusEl).not.toBeNull();
    expect(statusEl?.textContent).toMatch(/live/i);
  });

  it('displays received chat messages', async () => {
    render(<LiveChat streamId="stream-1" currentDisplayName="Alice" />);
    await waitForOpen();

    const ws = MockWebSocket.instances[0];
    act(() => {
      // The component expects envelope format: { type: 'message', data: ChatMessage }
      ws.receive({
        type: 'message',
        data: {
          id: 'msg-1',
          stream_id: 'stream-1',
          display_name: 'Bob',
          body: 'Hello world!',
          deleted: false,
          created_at: new Date().toISOString(),
        },
      });
    });

    await waitFor(() => {
      expect(screen.getByText('Hello world!')).toBeInTheDocument();
    });
    expect(screen.getByText(/Bob/)).toBeInTheDocument();
  });

  it('shows a message input for authenticated users', async () => {
    render(
      <LiveChat
        streamId="stream-1"
        currentUserId="u1"
        currentDisplayName="Alice"
      />
    );
    await waitForOpen();

    const input = screen.queryByRole('textbox');
    expect(input).not.toBeNull();
  });

  it('sends a message via WebSocket when submitted', async () => {
    const user = userEvent.setup();

    render(
      <LiveChat
        streamId="stream-1"
        currentUserId="u1"
        currentDisplayName="Alice"
      />
    );
    await waitForOpen();

    const ws = MockWebSocket.instances[0];
    const input = screen.getByRole('textbox');

    await user.type(input, 'Hey there!');
    await user.keyboard('{Enter}');

    expect(ws.sent.length).toBeGreaterThan(0);
    const sent = JSON.parse(ws.sent[0]);
    expect(sent.body).toBe('Hey there!');
    expect(sent.type).toBe('message');
  });

  it('opens a WebSocket with the stream ID in the URL', () => {
    render(<LiveChat streamId="stream-xyz" />);
    expect(MockWebSocket.instances[0].url).toContain('stream-xyz');
  });

  it('includes auth token in WebSocket URL when token is present', () => {
    localStorage.setItem('token', 'my-jwt-token');
    render(<LiveChat streamId="stream-1" currentUserId="u1" />);
    expect(MockWebSocket.instances[0].url).toContain('token=my-jwt-token');
  });

  it('renders system messages from history', async () => {
    render(<LiveChat streamId="stream-1" />);
    await waitForOpen();

    const ws = MockWebSocket.instances[0];
    act(() => {
      ws.receive({
        type: 'history',
        data: {
          messages: [
            {
              id: 'h1',
              stream_id: 'stream-1',
              display_name: 'Alice',
              body: 'First message',
              deleted: false,
              created_at: new Date().toISOString(),
            },
          ],
        },
      });
    });

    await waitFor(() => {
      expect(screen.getByText('First message')).toBeInTheDocument();
    });
  });
});
