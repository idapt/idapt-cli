package api

import (
	"context"
	"net/url"
	"strconv"
)

// PageParams represents cursor-based pagination parameters.
type PageParams struct {
	Limit         int
	StartingAfter string
	EndingBefore  string
}

// Query returns the pagination params as url.Values.
func (p PageParams) Query() url.Values {
	q := url.Values{}
	if p.Limit > 0 {
		q.Set("limit", strconv.Itoa(p.Limit))
	}
	if p.StartingAfter != "" {
		q.Set("starting_after", p.StartingAfter)
	}
	if p.EndingBefore != "" {
		q.Set("ending_before", p.EndingBefore)
	}
	return q
}

// PageResponse represents a paginated API response.
type PageResponse struct {
	Data    []map[string]interface{} `json:"data"`
	HasMore bool                     `json:"hasMore"`
	FirstID string                   `json:"firstId,omitempty"`
	LastID  string                   `json:"lastId,omitempty"`
}

// ListIterator provides automatic cursor-based pagination.
type ListIterator struct {
	client  *Client
	path    string
	params  PageParams
	extra   url.Values
	page    []map[string]interface{}
	idx     int
	hasMore bool
	started bool
	err     error
}

// NewListIterator creates a paginating iterator.
func NewListIterator(client *Client, path string, params PageParams, extra url.Values) *ListIterator {
	return &ListIterator{
		client:  client,
		path:    path,
		params:  params,
		extra:   extra,
		hasMore: true,
	}
}

// Next advances to the next item. Returns false when exhausted or on error.
func (it *ListIterator) Next(ctx context.Context) bool {
	if it.err != nil {
		return false
	}

	it.idx++
	if it.idx < len(it.page) {
		return true
	}

	if it.started && !it.hasMore {
		return false
	}

	// Fetch next page
	q := it.params.Query()
	for k, vs := range it.extra {
		for _, v := range vs {
			q.Add(k, v)
		}
	}

	var resp PageResponse
	if err := it.client.Get(ctx, it.path, q, &resp); err != nil {
		it.err = err
		return false
	}

	it.started = true
	it.page = resp.Data
	it.hasMore = resp.HasMore
	it.idx = 0

	if resp.LastID != "" {
		it.params.StartingAfter = resp.LastID
	}

	return len(it.page) > 0
}

// Item returns the current item.
func (it *ListIterator) Item() map[string]interface{} {
	if it.idx < len(it.page) {
		return it.page[it.idx]
	}
	return nil
}

// Err returns any error encountered during iteration.
func (it *ListIterator) Err() error {
	return it.err
}
