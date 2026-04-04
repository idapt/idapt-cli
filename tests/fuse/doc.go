// Package fuse_test contains integration tests for the FUSE mount system.
//
// These tests require:
// - A running idapt server (API accessible)
// - FUSE kernel module loaded (fusermount3 available)
// - Go test environment with IDAPT_API_URL and IDAPT_API_KEY set
//
// Run with:
//
//	go test -tags=integration -v ./tests/fuse/...
//
// The tests create temporary mount points, exercise FUSE operations,
// and verify server-side state via API calls.
package fuse_test
