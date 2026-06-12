import { useEffect, useMemo, useRef, useState } from 'react';
import SearchAutocomplete from './SearchAutocomplete';

type SearchMediaType = 'vod' | 'live';
type SearchDurationFilter = 'short' | 'medium' | 'long';
type SearchUploadDateFilter = 'today' | 'week' | 'month' | 'year';
type SearchSort = 'relevance' | 'recent' | 'views' | 'engagement';

type SearchResultItem = {
  id: string;
  title: string;
  description: string;
  owner_display_name: string;
  tags: string[];
  media_type: SearchMediaType;
  duration_seconds: number | null;
  view_count: number;
  like_count: number;
  published_at: string | null;
  score: number;
};

type APIResponse<T> = {
  success: boolean;
  data: T;
};

type SearchResult = {
  items: SearchResultItem[];
  total: number;
  query: {
    q?: string;
    limit?: number;
    offset?: number;
    duration?: SearchDurationFilter;
    upload_date?: SearchUploadDateFilter;
    media_type?: SearchMediaType;
    owner_id?: string;
    sort?: SearchSort;
  };
};

type Filters = {
  duration: SearchDurationFilter;
  upload_date: SearchUploadDateFilter;
  media_type: SearchMediaType;
  owner_id: string;
  sort: SearchSort;
};

const defaultFilters: Filters = {
  duration: '' as SearchDurationFilter,
  upload_date: '' as SearchUploadDateFilter,
  media_type: '' as SearchMediaType,
  owner_id: '',
  sort: 'relevance',
};

export default function SearchPage() {
  const [query, setQuery] = useState('');
  const [filters, setFilters] = useState<Filters>(defaultFilters);
  const [result, setResult] = useState<SearchResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const inputRef = useRef<HTMLInputElement>(null);
  const [showAutocomplete, setShowAutocomplete] = useState(false);

  const hasActiveFilters = useMemo(() => {
    return Boolean(filters.duration || filters.upload_date || filters.media_type || filters.owner_id);
  }, [filters]);

  function handleAutocompleteSelect(value: string) {
    setQuery(value);
    setShowAutocomplete(false);
    // Trigger search immediately by causing the query to re-evaluate
    setResult(null);
    inputRef.current?.focus();
  }

  useEffect(() => {
    const params = new URLSearchParams();
    if (query.trim()) params.set('q', query.trim());
    if (filters.duration) params.set('duration', filters.duration);
    if (filters.upload_date) params.set('upload_date', filters.upload_date);
    if (filters.media_type) params.set('media_type', filters.media_type);
    if (filters.owner_id.trim()) params.set('owner_id', filters.owner_id.trim());
    if (filters.sort) params.set('sort', filters.sort);
    params.set('limit', '20');

    let cancelled = false;
    setLoading(true);
    setError(null);

    fetch(`/api/v1/search?${params.toString()}`)
      .then((response) => {
        if (!response.ok) throw new Error('Search request failed');
        return response.json();
      })
      .then((data: APIResponse<SearchResult>) => {
        if (!cancelled && data.success) setResult(data.data);
      })
      .catch((err) => {
        if (!cancelled) setError(err instanceof Error ? err.message : String(err));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [query, filters]);

  function updateFilter<K extends keyof Filters>(key: K, value: Filters[K]) {
    setFilters((current) => ({ ...current, [key]: value }));
  }

  function clearFilters() {
    setFilters(defaultFilters);
  }

  return (
    <main className="app-shell">
      <section className="search-panel">
        <div>
          <p className="eyebrow">Search</p>
          <h1>Find videos</h1>
          <p className="muted">
            Search public ready videos with PostgreSQL full-text relevance, recency, view count, and engagement ranking.
          </p>
        </div>

        <form
          onSubmit={(e) => {
            e.preventDefault();
            setResult(null);
          }}
          className="search-form"
        >
          <div style={{ position: 'relative' }}>
            <input
              ref={inputRef}
              value={query}
              onChange={(e) => { setQuery(e.target.value); setShowAutocomplete(true); }}
              onFocus={() => setShowAutocomplete(true)}
              placeholder="Search by title, description, owner, or tags"
              aria-label="Search query"
            />
            {showAutocomplete && (
              <SearchAutocomplete query={query} onSelect={handleAutocompleteSelect} />
            )}
          </div>
          <button className="button" type="submit">Search</button>
        </form>

        <div className="search-filters">
          <label>
            Duration
            <select value={filters.duration} onChange={(e) => updateFilter('duration', e.target.value as SearchDurationFilter)}>
              <option value="">Any</option>
              <option value="short">Short (&lt; 4 min)</option>
              <option value="medium">Medium (4–20 min)</option>
              <option value="long">Long (20+ min)</option>
            </select>
          </label>

          <label>
            Upload date
            <select value={filters.upload_date} onChange={(e) => updateFilter('upload_date', e.target.value as SearchUploadDateFilter)}>
              <option value="">Any time</option>
              <option value="today">Today</option>
              <option value="week">This week</option>
              <option value="month">This month</option>
              <option value="year">This year</option>
            </select>
          </label>

          <label>
            Type
            <select value={filters.media_type} onChange={(e) => updateFilter('media_type', e.target.value as SearchMediaType)}>
              <option value="">Any</option>
              <option value="vod">VOD</option>
              <option value="live">Live</option>
            </select>
          </label>

          <label>
            Owner ID
            <input
              value={filters.owner_id}
              onChange={(e) => updateFilter('owner_id', e.target.value)}
              placeholder="Optional channel/user UUID"
            />
          </label>

          <label>
            Sort
            <select value={filters.sort} onChange={(e) => updateFilter('sort', e.target.value as SearchSort)}>
              <option value="relevance">Relevance</option>
              <option value="recent">Newest</option>
              <option value="views">Most viewed</option>
              <option value="engagement">Most engaged</option>
            </select>
          </label>

          {hasActiveFilters && (
            <button type="button" className="button button--secondary" onClick={clearFilters}>
              Clear filters
            </button>
          )}
        </div>
      </section>

      {error && <p className="error">{error}</p>}
      {loading && <p className="muted">Searching...</p>}

      {!loading && result && (
        <>
          <div className="search-summary">
            <strong>{result.total.toLocaleString()}</strong> result{result.total === 1 ? '' : 's'}
            {result.query.q && <span> for “{result.query.q}”</span>}
          </div>

          {result.items.length === 0 ? (
            <div className="card">
              <h2>No videos found</h2>
              <p>Try a broader query or remove one of the filters.</p>
            </div>
          ) : (
            <section className="video-grid">
              {result.items.map((item) => (
                <a key={item.id} href={`/watch/${item.id}`} className="video-card">
                  <div className="video-card-thumb" />
                  <div className="video-card-body">
                    <h3>{item.title}</h3>
                    {item.description && <p>{item.description}</p>}
                    <div className="video-card-meta">
                      <span>{item.owner_display_name}</span>
                      {item.duration_seconds !== null && <span>{formatDuration(item.duration_seconds)}</span>}
                      <span>{item.view_count.toLocaleString()} views</span>
                      <span>{item.media_type.toUpperCase()}</span>
                    </div>
                  </div>
                </a>
              ))}
            </section>
          )}
        </>
      )}
    </main>
  );
}

function formatDuration(seconds: number): string {
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = Math.floor(seconds % 60);
  if (h > 0) return `${h}:${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`;
  return `${m}:${s.toString().padStart(2, '0')}`;
}
