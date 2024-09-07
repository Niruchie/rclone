package filesystem

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
)

type Object struct {
	message    *telegram.MessageObj
	filesystem *Filesystem
	absolute   string
	relative   string
	fs.Object
}

// Creates a new object from a [telegram.MessageObj].
//
//	- The object is created with the absolute path of the file.
//
// [telegram.MessageObj]: https://pkg.go.dev/github.com/amarnathcjd/gogram/telegram#MessageObj
func NewObject(filesystem *Filesystem, message *telegram.MessageObj) Object {
	object := Object{
		absolute:   message.Message,
		filesystem: filesystem,
		message:    message,
	}

	object.relative = object.Remote()
	return object
}

// Creates a new object from a relative path.
//
//	- Commonly used to create a new object from [Fs.Put] method as no existing object is available.
//
// [Fs.Put]: https://pkg.go.dev/github.com/rclone/rclone/fs#Fs.Put
func NewObjectFromRelative(filesystem *Filesystem, relative string) Object {
	absolute := path.Join(filesystem.Root(), relative)
	message := &telegram.MessageObj{
		Message: absolute,
		ID:      0,
	}
	return NewObject(filesystem, message)
}

// Returns the absolute path of the object in the filesystem.
func (o Object) Directory() string {
	return path.Dir(o.absolute)
}

// Returns the relative path of the object in the filesystem.
func (o Object) DirectoryRelative() string {
	return path.Dir(o.relative)
}

// ? ----- Interface fs.Object methods -----

// SetModTime sets the metadata on the object to set the modification date.
//
// * Executing a touch would alter the modtime of the file.
//
// * Useful while testing the [Fs.NewObject] method.
//
// [Fs.NewObject]: https://pkg.go.dev/github.com/rclone/rclone/fs#Fs.NewObject
func (o Object) SetModTime(ctx context.Context, t time.Time) error {
	return nil
}

func (o Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	return nil, fs.ErrorNotImplemented
}

// Update in to the object with the modTime given of the given size.
//
// Read more about the method in [Object]
//
// [Object]: https://pkg.go.dev/github.com/rclone/rclone/fs#Object
func (o Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	return fs.ErrorNotImplemented
}

// Removes this object from the remote filesystem.
func (o Object) Remove(ctx context.Context) error {
	return fs.ErrorNotImplemented
}

// ? ----- Interface fs.Object : fs.ObjectInfo methods -----

// Hash returns the selected checksum of the file.
// If no checksum is available it returns "".
//
// ! Pending: Implementation should apply the hash algorithm to the file.
func (o Object) Hash(ctx context.Context, ty hash.Type) (string, error) {
	hasher := sha256.New()
	hasher.Write([]byte(o.absolute))
	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

// Storable says whether this object can be stored.
func (o Object) Storable() bool {
	return true
}

// ? ----- Interface fs.Object : fs.ObjectInfo : fs.DirEntry methods -----

// Fs returns read only access to the Fs that this object is part of.
func (o Object) Fs() fs.Info {
	return o.filesystem
}

// Remote returns the remote path.
//
// * The remote path is the path of the file relative to the root of the filesystem.
func (o Object) Remote() string {
	root := o.filesystem.Root()
	if o.message.Message == root {
		return path.Base(root)
	}

	trailRoot := TrailSlash(root)
	return strings.TrimPrefix(o.absolute, trailRoot)
}

// String returns a description of the Object
func (o Object) String() string {
	return fmt.Sprintf(
		"On filesystem %s, Object with telegram message ID %d, stored at path %s, relative to root %s, ",
		o.filesystem.Name(),
		o.message.ID,
		o.absolute,
		o.relative,
	)
}

// ModTime returns the modification date of the file.
// It should return a best guess if one isn't available.
//
// * The modification time of the file is the time when message created or edited.
func (o Object) ModTime(ctx context.Context) time.Time {
	time := time.Unix(int64(o.message.Date), 0)
	return time
}

// Size returns the size of the file.
//
// * The max size of the file is 2<<30 bytes = 2GB on this remote.
func (o Object) Size() int64 {
	return o.filesystem.MaxObjectSizeAccepted
}
