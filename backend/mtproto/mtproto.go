package mtproto

import (
	"github.com/rclone/rclone/backend/mtproto/configuration/options"
	"github.com/rclone/rclone/backend/mtproto/configuration"
	"github.com/rclone/rclone/fs"
)

// Register the MTProto backend.
//
// The definition of the backend is registered to the filesystem manager.
// It will be used to create a new instance of the backend.
// Parses the configuration and returns the configuration steps.
// Also, it will be used to mount a new filesystem to the rclone client.
func init() {
	fs.Register(
		&fs.RegInfo{
			Config:      configuration.Configuration,
			Options:     options.OptionList,
			NewFs:       configuration.Fs,
			Description: "MTProto",
			Name:        "mtproto",
		},
	)
}