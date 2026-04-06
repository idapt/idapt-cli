package fuse

import (
	"context"
	"log"
	"syscall"

	gofuse "github.com/hanwen/go-fuse/v2/fuse"
	"github.com/hanwen/go-fuse/v2/fs"
)

func (n *IdaptNode) Mkdir(ctx context.Context, name string, mode uint32, out *gofuse.EntryOut) (*fs.Inode, syscall.Errno) {
	entry, err := n.fuseFS.APIClient.CreateFolder(ctx, n.fuseFS.ProjectID, n.entry.ID, name)
	if err != nil {
		log.Printf("fuse-mkdir: failed to create %s: %v", name, err)
		if err == syscall.ENOENT {
			return nil, syscall.ENOENT
		}
		return nil, syscall.EIO
	}

	n.fuseFS.MetadataCache.Invalidate(n.childrenCacheKey())

	inode := n.childNode(ctx, entry)
	entry.fillAttrOut(&out.Attr)
	return inode, fs.OK
}

func (n *IdaptNode) Rmdir(ctx context.Context, name string) syscall.Errno {
	child := n.GetChild(name)
	if child == nil {
		return syscall.ENOENT
	}

	childNode, ok := child.Operations().(*IdaptNode)
	if !ok || !childNode.entry.IsFolder {
		return syscall.ENOTDIR
	}

	if err := n.fuseFS.APIClient.TrashFile(ctx, childNode.entry.ID); err != nil {
		log.Printf("fuse-rmdir: failed to remove %s: %v", name, err)
		return syscall.EIO
	}

	n.fuseFS.DiskCache.Evict(childNode.entry.ID)
	n.fuseFS.MetadataCache.Invalidate(n.childrenCacheKey())
	n.fuseFS.MetadataCache.Invalidate(n.lookupCacheKey(name))
	return fs.OK
}

func (n *IdaptNode) Unlink(ctx context.Context, name string) syscall.Errno {
	child := n.GetChild(name)
	if child == nil {
		return syscall.ENOENT
	}

	childNode, ok := child.Operations().(*IdaptNode)
	if !ok {
		return syscall.EIO
	}

	if childNode.entry.IsFolder {
		return syscall.EISDIR
	}

	if err := n.fuseFS.APIClient.TrashFile(ctx, childNode.entry.ID); err != nil {
		log.Printf("fuse-unlink: failed to remove %s: %v", name, err)
		return syscall.EIO
	}

	n.fuseFS.DiskCache.Evict(childNode.entry.ID)
	n.fuseFS.MetadataCache.Invalidate(n.childrenCacheKey())
	n.fuseFS.MetadataCache.Invalidate(n.lookupCacheKey(name))
	return fs.OK
}

func (n *IdaptNode) Rename(ctx context.Context, name string, newParent fs.InodeEmbedder, newName string, flags uint32) syscall.Errno {
	child := n.GetChild(name)
	if child == nil {
		return syscall.ENOENT
	}

	childNode, ok := child.Operations().(*IdaptNode)
	if !ok {
		return syscall.EIO
	}

	newParentNode, ok := newParent.(*IdaptNode)
	if !ok {
		return syscall.EIO
	}

	renamed := false
	if name != newName {
		if err := n.fuseFS.APIClient.RenameFile(ctx, childNode.entry.ID, newName); err != nil {
			log.Printf("fuse-rename: failed to rename %s → %s: %v", name, newName, err)
			return syscall.EIO
		}
		renamed = true
	}

	if n.entry.ID != newParentNode.entry.ID {
		if err := n.fuseFS.APIClient.MoveFile(ctx, childNode.entry.ID, newParentNode.entry.ID); err != nil {
			log.Printf("fuse-rename: failed to move %s to %s: %v", name, newParentNode.entry.ID, err)
			if renamed {
				if rbErr := n.fuseFS.APIClient.RenameFile(ctx, childNode.entry.ID, name); rbErr != nil {
					log.Printf("fuse-rename: rollback rename %s → %s also failed: %v", newName, name, rbErr)
				}
			}
			return syscall.EIO
		}
	}

	n.fuseFS.MetadataCache.Invalidate(n.childrenCacheKey())
	n.fuseFS.MetadataCache.Invalidate(n.lookupCacheKey(name))
	if n.entry.ID != newParentNode.entry.ID {
		n.fuseFS.MetadataCache.Invalidate(newParentNode.childrenCacheKey())
	}

	return fs.OK
}

func (n *IdaptNode) Symlink(ctx context.Context, target, name string, out *gofuse.EntryOut) (*fs.Inode, syscall.Errno) {
	entry, err := n.fuseFS.APIClient.CreateFile(ctx, n.fuseFS.ProjectID, n.entry.ID, name, []byte(target), "application/x-symlink")
	if err != nil {
		log.Printf("fuse-symlink: failed to create %s → %s: %v", name, target, err)
		return nil, syscall.EIO
	}

	n.fuseFS.MetadataCache.Invalidate(n.childrenCacheKey())

	inode := n.childNode(ctx, entry)
	entry.fillAttrOut(&out.Attr)
	return inode, fs.OK
}
