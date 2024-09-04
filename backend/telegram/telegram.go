package telegram

import (
	"github.com/rclone/rclone/backend/telegram/filesystem"
	"github.com/rclone/rclone/backend/telegram/types"
	"github.com/rclone/rclone/fs"
)

// Register the Telegram backend.
//
// The definition of the backend is registered to the filesystem manager.
// It will be used to create a new instance of the backend.
// Parses the configuration and returns the configuration steps.
// Also, it will be used to mount a new filesystem to the rclone client.
func init() {
	fs.Register(
		&fs.RegInfo{
			Config:      filesystem.Configuration,
			Options:     types.OptionList,
			NewFs:       filesystem.Fs,
			Description: "Telegram",
			Name:        "telegram",
		},
	)
}
