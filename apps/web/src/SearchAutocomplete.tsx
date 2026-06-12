import { useEffect, useRef, useState } from 'react';

type APIResponse<T> = {
  success: boolean;
  data: T;
};

export default function SearchAutocomplete({
  query,
  onSelect,
}: {
  query: string;
  onSelect: (value: string) => void;
}) {
  const [suggestions, setSuggestions] = useState<string[]>([]);
  const [open, setOpen] = useState(false);
  const [highlighted, setHighlighted] = useState(-1);
  const wrapperRef = useRef<HTMLDivElement>(null);

  // Debounced autocomplete fetch
  useEffect(() => {
    const trimmed = query.trim();
    if (!trimmed || trimmed.length < 2) {
      setSuggestions([]);
      setOpen(false);
      return;
    }

    const timer = setTimeout(() => {
      fetch(`/api/v1/search/autocomplete?q=${encodeURIComponent(trimmed)}&limit=8`)
        .then((r) => r.json())
        .then((data: APIResponse<string[]>) => {
          if (data.success && data.data.length > 0) {
            setSuggestions(data.data);
            setOpen(true);
            setHighlighted(-1);
          } else {
            setSuggestions([]);
            setOpen(false);
          }
        })
        .catch(() => {
          setSuggestions([]);
          setOpen(false);
        });
    }, 200);

    return () => clearTimeout(timer);
  }, [query]);

  // Click outside closes
  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (wrapperRef.current && !wrapperRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, []);

  function handleSelect(suggestion: string) {
    setOpen(false);
    onSelect(suggestion);
  }

  function handleKeyDown(e: React.KeyboardEvent) {
    if (!open || suggestions.length === 0) return;

    if (e.key === 'ArrowDown') {
      e.preventDefault();
      setHighlighted((prev) => (prev < suggestions.length - 1 ? prev + 1 : 0));
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setHighlighted((prev) => (prev > 0 ? prev - 1 : suggestions.length - 1));
    } else if (e.key === 'Enter' && highlighted >= 0) {
      e.preventDefault();
      handleSelect(suggestions[highlighted]);
    } else if (e.key === 'Escape') {
      setOpen(false);
    }
  }

  if (!open || suggestions.length === 0) return null;

  return (
    <div ref={wrapperRef} className="autocomplete-wrapper" onKeyDown={handleKeyDown}>
      <ul className="autocomplete-list" role="listbox">
        {suggestions.map((suggestion, i) => (
          <li
            key={i}
            role="option"
            aria-selected={i === highlighted}
            className={`autocomplete-item${i === highlighted ? ' autocomplete-item--active' : ''}`}
            onClick={() => handleSelect(suggestion)}
            onMouseEnter={() => setHighlighted(i)}
          >
            {suggestion}
          </li>
        ))}
      </ul>
    </div>
  );
}