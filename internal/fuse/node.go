package fuse

import (
	"context"
	"log"
	"syscall"
	"time"

	gofuse "github.com/hanwen/go-fuse/v2/fuse"
	"github.com/hanwen/go-fuse/v2/fs"
)

var _ = (fs.NodeGetattrer)((*IdaptNode)(nil))
var _ = (fs.NodeLookuper)((*IdaptNode)(nil))
var _ = (fs.NodeReaddirer)((*IdaptNode)(nil))
var _ = (fs.NodeCreater)((*IdaptNode)(nil))
var _ = (fs.NodeMkdirer)((*IdaptNode)(nil))
var _ = (fs.NodeRmdirer)((*IdaptNode)(nil))
var _ = (fs.NodeUnlinker)((*IdaptNode)(nil))
var _ = (fs.NodeRenamer)((*IdaptNode)(nil))
var _ = (fs.NodeSetattrer)((*IdaptNode)(nil))
var _ = (fs.NodeSymlinker)((*IdaptNode)(nil))
var _ = (fs.NodeReadlinker)((*IdaptNode)(nil))

type IdaptNode struct {
	fs.Inode

	entry  *FileEntry
	fuseFS *FuseFS
}

func (n *IdaptNode) Getattr(ctx context.Context, fh fs.FileHandle, out *gofuse.AttrOut) syscall.Errno {
	n.fillAttr(&out.Attr)
	return fs.OK
}

func (n *IdaptNode) fillAttr(attr *gofuse.Attr) {
	if n.entry.IsFolder {
		attr.Mode = 0755 | syscall.S_IFDIR
	} else if n.entry.MimeType == "application/x-symlink" {
		attr.Mode = 0777 | syscall.S_IFLNK
	} else {
		attr.Mode = 0644 | syscall.S_IFREG
	}

	attr.Size = uint64(n.entry.Size)
	attr.Mtime = uint64(n.entry.UpdatedAt.Unix())
	attr.Atime = uint64(n.entry.UpdatedAt.Unix())
	attr.Ctime = uint64(n.entry.CreatedAt.Unix())
	attr.Nlink = 1
	if n.entry.IsFolder {
		attr.Nlink = 2
	}
}

func (n *IdaptNode) childrenCacheKey() string {
	return "children:" + n.entry.ID
}

func (n *IdaptNode) lookupCacheKey(name string) string {
	return "lookup:" + n.entry.ID + ":" + name
}

func (n *IdaptNode) fetchChildren(ctx context.Context) ([]FileEntry, error) {
	cacheKey := n.childrenCacheKey()
	if cached, ok := n.fuseFS.MetadataCache.Get(cacheKey); ok {
		return cached.([]FileEntry), nil
	}

	children, err := n.fuseFS.APIClient.ListFiles(ctx, n.fuseFS.ProjectID, n.entry.ID)
	if err != nil {
		return nil, err
	}

	n.fuseFS.MetadataCache.Put(cacheKey, children)
	return children, nil
}

func (n *IdaptNode) childNode(ctx context.Context, entry *FileEntry) *fs.Inode {
	child := &IdaptNode{
		entry:  entry,
		fuseFS: n.fuseFS,
	}

	mode := uint32(syscall.S_IFREG)
	if entry.IsFolder {
		mode = syscall.S_IFDIR
	} else if entry.MimeType == "application/x-symlink" {
		mode = syscall.S_IFLNK
	}

	return n.NewPersistentInode(ctx, child, fs.StableAttr{Mode: mode})
}

func (n *IdaptNode) Lookup(ctx context.Context, name string, out *gofuse.EntryOut) (*fs.Inode, syscall.Errno) {
	cacheKey := n.lookupCacheKey(name)
	if cached, ok := n.fuseFS.MetadataCache.Get(cacheKey); ok {
		entry := cached.(*FileEntry)
		inode := n.childNode(ctx, entry)
		entry.fillAttrOut(&out.Attr)
		return inode, fs.OK
	}

	children, err := n.fetchChildren(ctx)
	if err != nil {
		log.Printf("fuse-lookup: error listing children of %s: %v", n.entry.ID, err)
		return nil, syscall.EIO
	}

	for i := range children {
		if children[i].Name == name {
			entry := &children[i]
			n.fuseFS.MetadataCache.Put(cacheKey, entry)
			inode := n.childNode(ctx, entry)
			entry.fillAttrOut(&out.Attr)
			return inode, fs.OK
		}
	}

	return nil, syscall.ENOENT
}

func (n *IdaptNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	children, err := n.fetchChildren(ctx)
	if err != nil {
		log.Printf("fuse-readdir: error listing children of %s: %v", n.entry.ID, err)
		return nil, syscall.EIO
	}

	entries := make([]gofuse.DirEntry, 0, len(children))
	for _, child := range children {
		mode := uint32(syscall.S_IFREG)
		if child.IsFolder {
			mode = syscall.S_IFDIR
		} else if child.MimeType == "application/x-symlink" {
			mode = syscall.S_IFLNK
		}

		entries = append(entries, gofuse.DirEntry{
			Name: child.Name,
			Mode: mode,
		})
	}

	return fs.NewListDirStream(entries), fs.OK
}

func (n *IdaptNode) Setattr(ctx context.Context, fh fs.FileHandle, in *gofuse.SetAttrIn, out *gofuse.AttrOut) syscall.Errno {
	if in.Valid&gofuse.FATTR_SIZE != 0 {
		newSize := int64(in.Size)
		if fh != nil {
			if ifh, ok := fh.(*IdaptFileHandle); ok {
				ifh.truncate(newSize)
			}
		} else {
			n.fuseFS.DiskCache.Evict(n.entry.ID)
		}
		n.entry.Size = newSize
	}

	n.fillAttr(&out.Attr)
	return fs.OK
}

func (n *IdaptNode) Readlink(ctx context.Context) ([]byte, syscall.Errno) {
	if n.entry.MimeType != "application/x-symlink" {
		return nil, syscall.EINVAL
	}

	reader, err := n.fuseFS.APIClient.DownloadFile(ctx, n.entry.ID)
	if err != nil {
		return nil, syscall.EIO
	}
	defer reader.Close()

	buf := make([]byte, 4096) // symlink targets are short
	nread, _ := reader.Read(buf)
	return buf[:nread], fs.OK
}

func (e *FileEntry) fillAttrOut(attr *gofuse.Attr) {
	if e.IsFolder {
		attr.Mode = 0755 | syscall.S_IFDIR
		attr.Nlink = 2
	} else if e.MimeType == "application/x-symlink" {
		attr.Mode = 0777 | syscall.S_IFLNK
		attr.Nlink = 1
	} else {
		attr.Mode = 0644 | syscall.S_IFREG
		attr.Nlink = 1
	}

	attr.Size = uint64(e.Size)
	attr.Mtime = uint64(e.UpdatedAt.Unix())
	attr.Atime = uint64(e.UpdatedAt.Unix())
	attr.Ctime = uint64(e.CreatedAt.Unix())
}

func (n *IdaptNode) Statfs(ctx context.Context, out *gofuse.StatfsOut) syscall.Errno {
	out.Blocks = 100 * 1024 * 1024 / 4096 // 100GB in 4K blocks
	out.Bfree = 50 * 1024 * 1024 / 4096   // 50GB free
	out.Bavail = out.Bfree
	out.Files = 1000000
	out.Ffree = 500000
	out.Bsize = 4096
	out.NameLen = 255
	return fs.OK
}

func (n *IdaptNode) Access(ctx context.Context, mask uint32) syscall.Errno {
	return fs.OK
}

func (n *IdaptNode) modifiedAt() time.Time {
	return n.entry.UpdatedAt
}
