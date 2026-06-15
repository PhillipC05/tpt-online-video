import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import CommentSection from '../CommentSection';

const mockFetch = vi.fn();

const sampleComment = {
  id: 'c1',
  video_id: 'vid1',
  user_id: 'u1',
  parent_id: null,
  body: 'Great video!',
  status: 'visible',
  owner: { id: 'u1', display_name: 'Alice' },
  like_count: 3,
  liked: false,
  replies: [],
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-01T00:00:00Z',
};

beforeEach(() => {
  vi.stubGlobal('fetch', mockFetch);
  localStorage.clear();
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

describe('CommentSection', () => {
  it('renders comments fetched from API', async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => [sampleComment],
    });

    render(<CommentSection videoId="vid1" currentUserId={null} />);

    await waitFor(() => {
      expect(screen.getByText('Great video!')).toBeInTheDocument();
    });
    expect(screen.getByText('Alice')).toBeInTheDocument();
  });

  it('renders empty state when there are no comments', async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => [],
    });

    render(<CommentSection videoId="vid1" currentUserId={null} />);

    await waitFor(() => {
      expect(screen.queryByText('Great video!')).not.toBeInTheDocument();
    });
  });

  it('shows comment form when user is logged in', async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => [],
    });

    render(<CommentSection videoId="vid1" currentUserId="u1" />);

    await waitFor(() => {
      // The comment textarea/form should exist for logged-in user
      const textarea = screen.queryByRole('textbox');
      expect(textarea).not.toBeNull();
    });
  });

  it('submits a new comment', async () => {
    const user = userEvent.setup();

    // First call: load comments (empty)
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => [],
    });
    // Second call: post comment
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        ...sampleComment,
        id: 'c2',
        body: 'New comment!',
      }),
    });
    // Third call: reload comments
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => [{ ...sampleComment, id: 'c2', body: 'New comment!' }],
    });

    render(<CommentSection videoId="vid1" currentUserId="u1" />);

    await waitFor(() => {
      expect(screen.getByRole('textbox')).toBeInTheDocument();
    });

    const textarea = screen.getByRole('textbox');
    await user.type(textarea, 'New comment!');

    const submitBtn = screen.getByRole('button', { name: /comment|post|submit/i });
    await user.click(submitBtn);

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledTimes(3);
    });

    // Verify post call was made with the right body
    const postCall = mockFetch.mock.calls[1];
    const body = JSON.parse(postCall[1].body);
    expect(body.body).toBe('New comment!');
  });

  it('shows like count on comments', async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => [sampleComment],
    });

    render(<CommentSection videoId="vid1" currentUserId={null} />);

    await waitFor(() => {
      // like count of 3 should appear somewhere
      expect(screen.getByText(/3/)).toBeInTheDocument();
    });
  });
});
