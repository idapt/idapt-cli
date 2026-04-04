package fuse

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"syscall"

	gofuse "github.com/hanwen/go-fuse/v2/fuse"
	"github.com/hanwen/go-fuse/v2/fs"
)

// retryOnEINTR retries an operation that returns (int, error) if interrupted by EINTR.
func retryOnEINTR(fn func() (int, error)) (int, error) {
	for {
		n, err := fn()
		if errors.Is(err, syscall.EINTR) {
			continue
		}
		return n, err
	}
}

// Compile-time interface checks for file handle
var _ = (fs.FileReader)((*IdaptFileHandle)(nil))
var _ = (fs.FileWriter)((*IdaptFileHandle)(nil))
var _ = (fs.FileFlusher)((*IdaptFileHandle)(nil))
var _ = (fs.FileReleaser)((*IdaptFileHandle)(nil))
var _ = (fs.FileGetattrer)((*IdaptFileHandle)(nil))

// IdaptFileHandle is the handle returned when a file is opened.
type IdaptFileHandle struct {
	entry       *FileEntry
	fuseFS      *FuseFS
	localPath   string   // path in disk cache
	dirty       bool     // has unsaved local changes
	openVersion int      // version at Open() for OCC conflict detection
	writeFile   *os.File // temp file for writes (nil until first write)
}

// Open creates a file handle. Cache-first: serves from local disk cache immediately
// without a server version check. OCC conflict detection happens on Flush() (write),
// not on Open() (read). This matches local Linux filesystem behavior — open() never
// checks if another process changed the file. SSE events invalidate the cache for
// remote changes; TTL (60s) is the safety net if SSE misses an event.
func (n *IdaptNode) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	if n.entry.IsFolder {
		return nil, 0, syscall.EISDIR
	}

	// Warn about SQLite files (unsafe on network mounts without exclusive locks)
	if ext := getExtension(n.entry.Name); ext == "db" || ext == "sqlite" {
		log.Printf("fuse-open: WARNING: SQLite file %s opened — unsafe without exclusive lock. Exclude *.db from sync.", n.entry.Name)
	}

	// Cache hit — serve immediately, no server round-trip.
	// openVersion from disk cache sidecar is used for OCC on Flush().
	cachedVersion := n.fuseFS.DiskCache.GetVersion(n.entry.ID)
	if cachedVersion >= 0 {
		return &IdaptFileHandle{
			entry:       n.entry,
			fuseFS:      n.fuseFS,
			openVersion: cachedVersion,
		}, 0, fs.OK
	}

	// Cache miss — download from server. This is the only case that hits the network.
	reader, err := n.fuseFS.APIClient.DownloadFile(ctx, n.entry.ID)
	if err != nil {
		log.Printf("fuse-open: download failed for %s: %v", n.entry.ID, err)
		return nil, 0, syscall.EIO
	}
	defer reader.Close()

	// Use version from FileEntry metadata (populated by Lookup → ListFiles API).
	// Falls back to fetching version only if metadata didn't include it.
	version := n.entry.Version
	if version == 0 {
		if v, verr := n.fuseFS.APIClient.GetFileVersion(ctx, n.entry.ID); verr == nil {
			version = v
		} else {
			version = 1 // safe default — Flush will get 409 if wrong, no data loss
		}
	}

	if _, err := n.fuseFS.DiskCache.Put(n.entry.ID, version, reader); err != nil {
		log.Printf("fuse-open: cache put failed for %s: %v", n.entry.ID, err)
	}

	return &IdaptFileHandle{
		entry:       n.entry,
		fuseFS:      n.fuseFS,
		openVersion: version,
	}, 0, fs.OK
}

// Read serves file content from the local disk cache.
func (fh *IdaptFileHandle) Read(ctx context.Context, dest []byte, off int64) (gofuse.ReadResult, syscall.Errno) {
	// If we have a local write buffer, read from it
	if fh.writeFile != nil {
		n, err := fh.writeFile.ReadAt(dest, off)
		if err != nil && err != io.EOF {
			return nil, syscall.EIO
		}
		return gofuse.ReadResultData(dest[:n]), fs.OK
	}

	// Read from disk cache using ReadAt for correct offset handling.
	// DiskCache.Get() returns an *os.File which supports ReadAt.
	reader, err := fh.fuseFS.DiskCache.Get(fh.entry.ID)
	if err != nil || reader == nil {
		return nil, syscall.EIO
	}
	defer reader.Close()

	// Use ReadAt for offset-based reads (os.File supports this)
	if f, ok := reader.(*os.File); ok {
		n, err := f.ReadAt(dest, off)
		if err != nil && err != io.EOF {
			return nil, syscall.EIO
		}
		return gofuse.ReadResultData(dest[:n]), fs.OK
	}

	// Fallback: seek + read for non-file readers
	if seeker, ok := reader.(io.ReadSeeker); ok {
		if _, err := seeker.Seek(off, io.SeekStart); err != nil {
			return nil, syscall.EIO
		}
	}

	n, readErr := reader.Read(dest)
	if readErr != nil && readErr != io.EOF {
		return nil, syscall.EIO
	}
	return gofuse.ReadResultData(dest[:n]), fs.OK
}

// Write buffers data to a local temp file.
func (fh *IdaptFileHandle) Write(ctx context.Context, data []byte, off int64) (uint32, syscall.Errno) {
	if fh.writeFile == nil {
		// Create temp file for writes
		tmpFile, err := os.CreateTemp("", "idapt-fuse-*")
		if err != nil {
			return 0, syscall.EIO
		}
		fh.writeFile = tmpFile

		// Copy existing cached content into write buffer
		reader, err := fh.fuseFS.DiskCache.Get(fh.entry.ID)
		if err == nil && reader != nil {
			io.Copy(fh.writeFile, reader)
			reader.Close()
		}
	}

	n, err := retryOnEINTR(func() (int, error) {
		return fh.writeFile.WriteAt(data, off)
	})
	if err != nil {
		// Ignore SIGPIPE for writes to broken pipes
		if errors.Is(err, syscall.EPIPE) {
			return 0, syscall.EPIPE
		}
		return 0, syscall.EIO
	}

	fh.dirty = true
	return uint32(n), fs.OK
}

// Flush uploads dirty content to the server with OCC.
func (fh *IdaptFileHandle) Flush(ctx context.Context) syscall.Errno {
	if !fh.dirty || fh.writeFile == nil {
		return fs.OK
	}

	// Read all content from temp file
	if _, err := fh.writeFile.Seek(0, io.SeekStart); err != nil {
		return syscall.EIO
	}

	content, err := io.ReadAll(fh.writeFile)
	if err != nil {
		return syscall.EIO
	}

	// Upload with OCC version check — use presigned S3 for large files
	var uploadErr error
	if len(content) > LargeFileThreshold {
		mimeType := fh.entry.MimeType
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		uploadErr = fh.fuseFS.APIClient.UploadLargeFile(ctx, fh.entry.ID, content, mimeType, fh.openVersion)
	} else {
		uploadErr = fh.fuseFS.APIClient.UpdateFileContent(ctx, fh.entry.ID, string(content), fh.openVersion)
	}
	if err := uploadErr; err != nil {
		if err == syscall.ESTALE {
			// OCC conflict — save as .conflict file
			conflictPath := fh.fuseFS.DiskCache.LocalPath(fh.entry.Name + ".conflict")
			if writeErr := os.WriteFile(conflictPath, content, 0644); writeErr != nil {
				log.Printf("fuse-flush: failed to save conflict file: %v", writeErr)
			} else {
				log.Printf("fuse-flush: OCC conflict on %s — saved as %s", fh.entry.Name, conflictPath)
			}
			return syscall.ESTALE
		}
		log.Printf("fuse-flush: upload failed for %s: %v", fh.entry.ID, err)
		// Keep dirty in disk cache for background flusher retry
		fh.fuseFS.DiskCache.MarkDirty(fh.entry.ID)
		return syscall.EIO
	}

	// Update cache with new version
	fh.fuseFS.DiskCache.Put(fh.entry.ID, fh.openVersion+1, bytesReader(content))
	fh.fuseFS.MetadataCache.InvalidatePrefix("children:" + ptrToStr(fh.entry.ParentID))

	fh.dirty = false
	log.Printf("fuse-flush: uploaded %s (v%d → v%d)", fh.entry.Name, fh.openVersion, fh.openVersion+1)
	return fs.OK
}

// Release cleans up the file handle.
func (fh *IdaptFileHandle) Release(ctx context.Context) syscall.Errno {
	if fh.writeFile != nil {
		path := fh.writeFile.Name()
		fh.writeFile.Close()
		os.Remove(path)
	}
	return fs.OK
}

// Getattr returns file attributes for an open file.
func (fh *IdaptFileHandle) Getattr(ctx context.Context, out *gofuse.AttrOut) syscall.Errno {
	out.Mode = 0644 | syscall.S_IFREG
	out.Size = uint64(fh.entry.Size)
	out.Mtime = uint64(fh.entry.UpdatedAt.Unix())
	out.Atime = uint64(fh.entry.UpdatedAt.Unix())
	out.Ctime = uint64(fh.entry.CreatedAt.Unix())
	out.Nlink = 1

	// If we have a write buffer, use its size
	if fh.writeFile != nil {
		info, err := fh.writeFile.Stat()
		if err == nil {
			out.Size = uint64(info.Size())
		}
	}

	return fs.OK
}

// ptrToStr safely dereferences a *string.
func ptrToStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// bytesReader wraps a byte slice as an io.Reader.
type bytesReaderWrapper struct {
	data []byte
	pos  int
}

func bytesReader(data []byte) io.Reader {
	return &bytesReaderWrapper{data: data}
}

func (r *bytesReaderWrapper) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

// CreateFile implements file creation from the FUSE side.
func (n *IdaptNode) Create(ctx context.Context, name string, flags uint32, mode uint32, out *gofuse.EntryOut) (inode *fs.Inode, fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	// Detect MIME type from extension (simplified)
	mimeType := "application/octet-stream"
	if ext := getExtension(name); ext != "" {
		mimeType = mimeFromExt(ext)
	}

	entry, err := n.fuseFS.APIClient.CreateFile(ctx, n.fuseFS.ProjectID, n.entry.ID, name, []byte{}, mimeType)
	if err != nil {
		log.Printf("fuse-create: failed to create %s: %v", name, err)
		return nil, nil, 0, syscall.EIO
	}

	// Invalidate parent children cache
	n.fuseFS.MetadataCache.Invalidate(n.childrenCacheKey())

	inode = n.childNode(ctx, entry)
	entry.fillAttrOut(&out.Attr)

	handle := &IdaptFileHandle{
		entry:       entry,
		fuseFS:      n.fuseFS,
		openVersion: entry.Version,
	}

	return inode, handle, 0, fs.OK
}

// getExtension extracts the file extension from a name.
func getExtension(name string) string {
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '.' {
			return name[i+1:]
		}
	}
	return ""
}

// mimeFromExt returns a MIME type from a file extension.
func mimeFromExt(ext string) string {
	switch ext {
	case "txt":
		return "text/plain"
	case "md":
		return "text/markdown"
	case "json":
		return "application/json"
	case "js":
		return "application/javascript"
	case "ts":
		return "application/typescript"
	case "html":
		return "text/html"
	case "css":
		return "text/css"
	case "xml":
		return "application/xml"
	case "yaml", "yml":
		return "text/yaml"
	case "sh":
		return "text/x-shellscript"
	case "py":
		return "text/x-python"
	case "go":
		return "text/x-go"
	case "rs":
		return "text/x-rust"
	case "png":
		return "image/png"
	case "jpg", "jpeg":
		return "image/jpeg"
	case "gif":
		return "image/gif"
	case "svg":
		return "image/svg+xml"
	case "pdf":
		return "application/pdf"
	default:
		return fmt.Sprintf("application/x-%s", ext)
	}
}
