package api

import (
	"context"
	"net/url"
	"strconv"
)

type PageParams struct {
	Limit         int
	StartingAfter string
	EndingBefore  string
}

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

type PageResponse struct {
	Data    []map[string]interface{} `json:"data"`
	HasMore bool                     `json:"hasMore"`
	FirstID string                   `json:"firstId,omitempty"`
	LastID  string                   `json:"lastId,omitempty"`
}

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

func NewListIterator(client *Client, path string, params PageParams, extra url.Values) *ListIterator {
	return &ListIterator{
		client:  client,
		path:    path,
		params:  params,
		extra:   extra,
		hasMore: true,
	}
}

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

func (it *ListIterator) Item() map[string]interface{} {
	if it.idx < len(it.page) {
		return it.page[it.idx]
	}
	return nil
}

func (it *ListIterator) Err() error {
	return it.err
}
