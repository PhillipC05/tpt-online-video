package handlers

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/tpt-online-video/packages/search"
)

type SearchHandler struct {
	logger *slog.Logger
	search search.Provider
}

func NewSearchHandler(logger *slog.Logger, provider search.Provider) *SearchHandler {
	return &SearchHandler{logger: logger, search: provider}
}

func (h *SearchHandler) Autocomplete(w http.ResponseWriter, r *http.Request) {
	prefix := r.URL.Query().Get("q")
	if prefix == "" {
		writeJSON(w, http.StatusOK, []string{})
		return
	}

	limit, _ := parseOptionalInt(r.URL.Query().Get("limit"), 10)

	suggestions, err := h.search.Autocomplete(r.Context(), prefix, limit)
	if err != nil {
		h.logger.Error("search autocomplete", "error", err)
		writeError(w, http.StatusInternalServerError, "autocomplete failed")
		return
	}

	writeJSON(w, http.StatusOK, suggestions)
}

func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	qs := r.URL.Query()

	limit, err := parseOptionalInt(qs.Get("limit"), 20)
	if err != nil {
		writeError(w, http.StatusBadRequest, "limit must be an integer")
		return
	}
	offset, err := parseOptionalInt(qs.Get("offset"), 0)
	if err != nil {
		writeError(w, http.StatusBadRequest, "offset must be an integer")
		return
	}

	query := search.Query{
		Text:       qs.Get("q"),
		Limit:      limit,
		Offset:     offset,
		Duration:   search.DurationFilter(qs.Get("duration")),
		UploadDate: search.UploadDateFilter(qs.Get("upload_date")),
		MediaType:  search.MediaType(qs.Get("media_type")),
		OwnerID:    qs.Get("owner_id"),
		Sort:       search.Sort(qs.Get("sort")),
	}

	result, err := h.search.Search(r.Context(), query)
	if err != nil {
		h.logger.Error("search videos", "error", err)
		writeError(w, http.StatusInternalServerError, "search failed")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func parseOptionalInt(value string, fallback int) (int, error) {
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	return parsed, nil
}
