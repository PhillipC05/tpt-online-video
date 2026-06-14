import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import EmptyState from '../components/EmptyState';

describe('EmptyState', () => {
  it('renders title and description', () => {
    render(<EmptyState title="No videos" description="Upload one to get started." />);
    expect(screen.getByText('No videos')).toBeInTheDocument();
    expect(screen.getByText('Upload one to get started.')).toBeInTheDocument();
  });

  it('renders a link action', () => {
    render(<EmptyState title="Empty" action={{ label: 'Go home', href: '/' }} />);
    const link = screen.getByRole('link', { name: 'Go home' });
    expect(link).toHaveAttribute('href', '/');
  });

  it('renders a button action and calls onClick', async () => {
    const user = userEvent.setup();
    const onClick = vi.fn();
    render(<EmptyState title="Empty" action={{ label: 'Retry', onClick }} />);
    await user.click(screen.getByRole('button', { name: 'Retry' }));
    expect(onClick).toHaveBeenCalledOnce();
  });

  it('has role=status for screen readers', () => {
    render(<EmptyState title="Nothing here" />);
    expect(screen.getByRole('status')).toBeInTheDocument();
  });
});
