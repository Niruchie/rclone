package filesystem

import (
	"context"
	"time"

	"github.com/rclone/rclone/fs"
)

type Object struct {
	fs.Object
	absolute   string
	relative   string
	filesystem *Filesystem
}

// ? ----- Interface fs.Object : fs.ObjectInfo methods -----

// ? SetModTime sets the modification time of the object.
// * Executing a touch would alter the modtime of the file.
func (o Object) SetModTime(ctx context.Context, t time.Time) error {
	return nil
}