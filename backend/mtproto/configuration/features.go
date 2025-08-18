package configuration

import (
	"context"

	_ "github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs"
)

// NewMTProtoFeatures creates a new feature set for the backend.
func NewMTProtoFeatures(f *Filesystem) *fs.Features {
	return &fs.Features{
		CaseInsensitive:          false,
		DuplicateFiles:           false,
		ReadMimeType:             false,          // TODO
		WriteMimeType:            false,
		CanHaveEmptyDirectories:  true,
		BucketBased:              true,
		BucketBasedRootOK:        false,
		SetTier:                  false,          // Unknown
		GetTier:                  false,          // Unknown
		ServerSideAcrossConfigs:  false,          // TODO
		IsLocal:                  false,
		SlowModTime:              true,
		SlowHash:                 true,
		ReadMetadata:             true,
		WriteMetadata:            false,
		UserMetadata:             true,
		ReadDirMetadata:          true,
		WriteDirMetadata:         false,
		WriteDirSetModTime:       false,
		UserDirMetadata:          false,
		DirModTimeUpdatesOnWrite: false,          // TODO
		FilterAware:              true,
		PartialUploads:           false,          // TODO
		NoMultiThreading:         false,
		Overlay:                  true,           // Wrap mtprotobot
		ChunkWriterDoesntSeek:    false,

		// ? ----- Implements the following methods -----
		About: f.Usage,
	}
}

// Usage gets the quota information for the Fs.
//
// Definition:
//
//	Usage(ctx context.Context) (*fs.Usage, error)
//
// Parameters:
//
//	ctx context.Context - The context for the request.
//
// Returns:
//
//	*fs.Usage - The usage information.
//	error - If an error occurred.
func (f *Filesystem) Usage(ctx context.Context) (*fs.Usage, error) {
	return &fs.Usage{}, nil
}
