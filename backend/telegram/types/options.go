package types

import "github.com/rclone/rclone/fs"

var SessionStringEmpty string = ""

// ? Options defines the configuration for this backend
type Options struct {
	AppId            int32         `config:"app_id"`
	AppHash          string        `config:"app_hash"`
	BotToken         string        `config:"bot_token"`
	ChunkSize        fs.SizeSuffix `config:"chunk_size"`
	ChannelId        int64         `config:"channel_id"`
	PublicKey        string        `config:"public_key"`
	PhoneNumber      string        `config:"phone_number"`
	StringSession    string        `config:"string_session"`
	MaxConnections   int           `config:"max_connections"`
}