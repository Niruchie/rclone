package types

import "github.com/rclone/rclone/fs"

// ? Options defines the configuration for this backend
type Options struct {
	AppId          int32         `config:"app_id"`
	AppHash        string        `config:"app_hash"`
	BotToken       string        `config:"bot_token"`
	ChunkSize      fs.SizeSuffix `config:"chunk_size"`
	ChannelId      int64         `config:"channel_id"`
	PublicKey      string        `config:"public_key"`
	PhoneNumber    string        `config:"phone_number"`
	StringSession  string        `config:"string_session"`
	MaxConnections int           `config:"max_connections"`
}

// ? Constants to be used in the backend.
var (
	// An empty string for the session. | Generated after rclone configuration.
	SessionStringEmpty string = ""

	// The maximum object size accepted by the backend.
	MaxObjectSizeAccepted int64 = 2 << 30

	// A list of options for the backend. | Contains the configuration options for the backend.
	OptionList []fs.Option = []fs.Option{
		{
			Help:      "Phone number for Telegram API",
			Name:      "phone_number",
			Advanced:  false,
			Required:  true,
			Sensitive: true,
		},
		{
			Help:      "App ID for Telegram API",
			Name:      "app_id",
			Advanced:  false,
			Required:  true,
			Sensitive: true,
		}, {
			Help:      "App Hash for Telegram API",
			Name:      "app_hash",
			Advanced:  false,
			Required:  true,
			Sensitive: true,
		}, {
			Help:      "Bot Token for Telegram API",
			Name:      "bot_token",
			Advanced:  false,
			Required:  true,
			Sensitive: true,
		}, {
			Help:      "Public Key for Telegram API (Should be base64 encoded, PEM format)",
			Name:      "public_key",
			Advanced:  false,
			Required:  true,
			Sensitive: true,
		}, {
			Help:     "Display other channel origins",
			Name:     "display_channel",
			Advanced: true,
			Default:  false,
			Examples: []fs.OptionExample{
				{Value: "true", Help: "Yes, display uploads from other channels"},
				{Value: "false", Help: "No, only display uploads from the selected channel"},
			},
		}, {
			Help:     "Maximum number of connections to use",
			Name:     "max_connections",
			Provider: "telegram",
			Advanced: true,
			Default:  10,
		}, {
			Help:     `Files will be uploaded in chunks this size. Note that these chunks might be buffered in memory, increasing them might increase memory use.`,
			Name:     "chunk_size",
			Advanced: true,
			Default:  512 * fs.Mebi,
			Examples: []fs.OptionExample{
				{Value: "512", Help: "512 MiB (Fastest)"},
				{Value: "256", Help: "256 MiB (Faster)"},
				{Value: "128", Help: "128 MiB (Fast)"},
				{Value: "64", Help: "64 MiB (Low priority)"},
				{Value: "32", Help: "32 MiB (High memory usage)"},
				{Value: "16", Help: "16 MiB (Intensive memory usage)"},
			},
		},
	}
)
