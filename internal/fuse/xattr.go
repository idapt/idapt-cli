package fuse

import (
	"context"
	"fmt"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
)

const (
	xattrPrefix    = "user.idapt."
	xattrResID     = xattrPrefix + "resource_id"
	xattrProjectID = xattrPrefix + "project_id"
	xattrCreatedBy = xattrPrefix + "created_by"
	xattrIcon      = xattrPrefix + "icon"
	xattrPrompt    = xattrPrefix + "prompt"
	xattrDuration  = xattrPrefix + "duration_ms"
	xattrPublic    = xattrPrefix + "public_access"
	xattrVersion   = xattrPrefix + "version"
)

var allXattrKeys = []string{
	xattrResID, xattrProjectID, xattrCreatedBy,
	xattrIcon, xattrPrompt, xattrDuration,
	xattrPublic, xattrVersion,
}

var writableXattrs = map[string]bool{
	xattrIcon:   true,
	xattrPrompt: true,
	xattrPublic: true,
}

func (n *IdaptNode) Listxattr(ctx context.Context, dest []byte) (uint32, syscall.Errno) {
	var size uint32
	for _, key := range allXattrKeys {
		size += uint32(len(key)) + 1
	}

	if len(dest) == 0 {
		return size, fs.OK
	}

	if uint32(len(dest)) < size {
		return 0, syscall.ERANGE
	}

	offset := 0
	for _, key := range allXattrKeys {
		copy(dest[offset:], key)
		offset += len(key)
		dest[offset] = 0 // null terminator
		offset++
	}

	return size, fs.OK
}

func (n *IdaptNode) Getxattr(ctx context.Context, attr string, dest []byte) (uint32, syscall.Errno) {
	value, ok := n.getXattrValue(attr)
	if !ok {
		return 0, syscall.ENODATA
	}

	data := []byte(value)
	if len(dest) == 0 {
		return uint32(len(data)), fs.OK
	}

	if len(dest) < len(data) {
		return 0, syscall.ERANGE
	}

	copy(dest, data)
	return uint32(len(data)), fs.OK
}

func (n *IdaptNode) Setxattr(ctx context.Context, attr string, data []byte, flags uint32) syscall.Errno {
	if !writableXattrs[attr] {
		return syscall.EPERM
	}

	value := string(data)

	var field string
	switch attr {
	case xattrIcon:
		field = "icon"
	case xattrPrompt:
		field = "prompt"
	case xattrPublic:
		field = "publicAccess"
	default:
		return syscall.EPERM
	}

	if err := n.fuseFS.APIClient.client.Patch(ctx, "/api/files/"+n.entry.ID, map[string]interface{}{
		field: value,
	}, nil); err != nil {
		return syscall.EIO
	}

	switch attr {
	case xattrIcon:
		n.entry.Icon = value
	case xattrPrompt:
		n.entry.Prompt = value
	case xattrPublic:
		n.entry.PublicAccess = value
	}

	n.fuseFS.MetadataCache.Invalidate("lookup:" + ptrToStr(n.entry.ParentID) + ":" + n.entry.Name)
	return fs.OK
}

func (n *IdaptNode) Removexattr(ctx context.Context, attr string) syscall.Errno {
	return syscall.EPERM
}

func (n *IdaptNode) getXattrValue(attr string) (string, bool) {
	switch attr {
	case xattrResID:
		return n.entry.ResourceID, true
	case xattrProjectID:
		return n.entry.ProjectID, true
	case xattrCreatedBy:
		return n.entry.CreatedByActorID, true
	case xattrIcon:
		return n.entry.Icon, true
	case xattrPrompt:
		return n.entry.Prompt, true
	case xattrDuration:
		return fmt.Sprintf("%d", n.entry.DurationMs), true
	case xattrPublic:
		return n.entry.PublicAccess, true
	case xattrVersion:
		return fmt.Sprintf("%d", n.entry.Version), true
	default:
		return "", false
	}
}
