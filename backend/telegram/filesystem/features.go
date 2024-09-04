package filesystem

import (
	"context"

	"github.com/rclone/rclone/fs"
)

// NewTelegramFeatures creates a new feature set for the backend.
func NewTelegramFeatures(f * Filesystem) *fs.Features {
	return &fs.Features{
		CaseInsensitive:          false,
		DuplicateFiles:           false,
		ReadMimeType:             false,
		WriteMimeType:            false,
		CanHaveEmptyDirectories:  true,
		BucketBased:              false,
		BucketBasedRootOK:        false,
		SetTier:                  false,
		GetTier:                  false,
		ServerSideAcrossConfigs:  false,
		IsLocal:                  false,
		SlowModTime:              false,
		SlowHash:                 true,
		ReadMetadata:             true,
		WriteMetadata:            false,
		UserMetadata:             true,
		ReadDirMetadata:          true,
		WriteDirMetadata:         false,
		WriteDirSetModTime:       false,
		UserDirMetadata:          false,
		DirModTimeUpdatesOnWrite: false,
		FilterAware:              true,
		PartialUploads:           false,
		NoMultiThreading:         false,
		Overlay:                  false,
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
func (f * Filesystem) Usage(ctx context.Context) (*fs.Usage, error) {
	return &fs.Usage{}, nil
}
