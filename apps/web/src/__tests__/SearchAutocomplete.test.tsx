import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import SearchAutocomplete from '../SearchAutocomplete';

const mockFetch = vi.fn();

beforeEach(() => {
  vi.stubGlobal('fetch', mockFetch);
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

function mockAutocompleteResponse(suggestions: string[]) {
  mockFetch.mockResolvedValue({
    ok: true,
    json: async () => ({ success: true, data: suggestions }),
  });
}

// Wait past the 200ms debounce
function wait(ms = 250) {
  return new Promise<void>((r) => setTimeout(r, ms));
}

describe('SearchAutocomplete', () => {
  it('renders nothing when query is empty', () => {
    const { container } = render(
      <SearchAutocomplete query="" onSelect={vi.fn()} />
    );
    expect(container.firstChild).toBeNull();
  });

  it('renders nothing when query is shorter than 2 chars', () => {
    const { container } = render(
      <SearchAutocomplete query="a" onSelect={vi.fn()} />
    );
    expect(container.firstChild).toBeNull();
  });

  it('shows suggestions after debounce when query >= 2 chars', async () => {
    mockAutocompleteResponse(['react tutorial', 'react hooks']);

    render(<SearchAutocomplete query="re" onSelect={vi.fn()} />);

    // Nothing yet
    expect(screen.queryByRole('listbox')).toBeNull();

    await wait();

    await waitFor(() => {
      expect(screen.getByRole('listbox')).toBeInTheDocument();
    }, { timeout: 2000 });

    expect(screen.getByText('react tutorial')).toBeInTheDocument();
    expect(screen.getByText('react hooks')).toBeInTheDocument();
  });

  it('hides suggestions when API returns empty array', async () => {
    mockAutocompleteResponse([]);

    render(<SearchAutocomplete query="xyz" onSelect={vi.fn()} />);

    await wait();

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalled();
    }, { timeout: 2000 });

    expect(screen.queryByRole('listbox')).toBeNull();
  });

  it('calls onSelect with the suggestion when clicked', async () => {
    const onSelect = vi.fn();
    const user = userEvent.setup();
    mockAutocompleteResponse(['javascript']);

    render(<SearchAutocomplete query="jav" onSelect={onSelect} />);

    await wait();

    await waitFor(() => {
      expect(screen.getByText('javascript')).toBeInTheDocument();
    }, { timeout: 2000 });

    await user.click(screen.getByText('javascript'));
    expect(onSelect).toHaveBeenCalledWith('javascript');
  });

  it('hides suggestions on fetch error', async () => {
    mockFetch.mockRejectedValue(new Error('network error'));

    render(<SearchAutocomplete query="react" onSelect={vi.fn()} />);

    await wait();

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalled();
    }, { timeout: 2000 });

    expect(screen.queryByRole('listbox')).toBeNull();
  });

  it('encodes the query in the fetch URL', async () => {
    mockAutocompleteResponse(['c++ tutorial']);

    render(<SearchAutocomplete query="c++" onSelect={vi.fn()} />);

    await wait();

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalled();
    }, { timeout: 2000 });

    const url: string = mockFetch.mock.calls[0][0];
    expect(url).toContain('c%2B%2B');
  });
});
