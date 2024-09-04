package types

import (
	"time"

	"github.com/rclone/rclone/fs"
)

// ? Options defines the configuration for this backend
type Options struct {
	AppId          int32         `config:"app_id"`
	AppHash        string        `config:"app_hash"`
	BotToken       string        `config:"bot_token"`
	ChunkSize      fs.SizeSuffix `config:"chunk_size"`
	ChannelId      int64         `config:"channel_id"`
	PublicKey      string        `config:"public_key"`
	TestServer     bool          `config:"test_server"`
	PhoneNumber    string        `config:"phone_number"`
	StringSession  string        `config:"string_session"`
	MaxConnections int           `config:"max_connections"`
}

// ? Constants to be used in the backend.
var (
	// Session to use when login is performed.
	// Also used to login to Telegram Bot API.
	//
	//	- Generated after rclone configuration.
	//	- An empty string for the session.
	SessionStringEmpty string = ""

	// Time to wait for the session to avoid rate limiting.
	//	- The Telegram API message is [FLOOD_WAIT_X].
	//	- Should wait X seconds before login again.
	//	- Test DC servers have a lower rate limit.
	//
	// [FLOOD_WAIT_X]: https://core.telegram.org/api/errors#420-flood
	SessionFloodWait time.Duration = time.Second * 5

	// The part size for the multipart upload.
	// TODO: Convert this to a advanced option in the backend.
	//
	// [Telegram API Documentation | Files] - Default is 512 KB
	//
	// [Telegram API Documentation | Files]: https://core.telegram.org/api/files
	MaxPartSizeAccepted int64 = 512 << 10

	// The maximum object size accepted by the backend.
	// TODO: Convert this to a advanced option in the backend.
	//
	// [Telegram API Documentation | Files] - Default is 2 GiB (non-premium)
	//
	// [Telegram API Documentation | Files]: https://core.telegram.org/api/files
	MaxObjectSizeAccepted int64 = 2 << 30

	// When using Streamed Uploads, use unknown size of document to upload.
	// This forces the backend to use the `upload.saveBigFilePart` method.
	// Even if the size is known or less than the required size.
	//
	// [Telegram API Documentation | Files] - Default is -1
	//
	// [upload.saveBigFilePart] - Upload big files to the server.
	//
	// [Telegram API Documentation | Files]: https://core.telegram.org/api/files
	// [upload.saveBigFilePart]: https://core.telegram.org/method/upload.saveBigFilePart
	StreamedUploadUnknownSize int32 = -1

	// When using Streamed Downloads, use precise size for the download part size.
	// The request will download the exact size and assing it to the reader.
	// Non-buffered bytes will be cached in the reader for next read.
	MaxDownloadPreciseSize int32 = 1 << 20

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
			Help:      "Whether this Telegram account should use MTProto API and Bot API development servers",
			Name:      "test_server",
			Required:  true,
			Default:   false,
			Advanced:  true,
			Exclusive: true,
			Examples: []fs.OptionExample{
				{
					Value:    "false",
					Provider: "Telegram Production DC",
					Help:     "No, use the account on production DC for Telegram API",
				},
				{
					Value:    "true",
					Provider: "Telegram Testing DC",
					Help:     "Yes, use the testing account for Telegram API",
				},
			},
		}, {
			Help:     "Maximum number of connections to use. Can lead to flood rate limiting if too high",
			Name:     "max_connections",
			Provider: "telegram",
			Required: true,
			Advanced: true,
			Default:  10,
		}, {
			Help:      `Files will be uploaded in chunks this size. Note that these chunks might be buffered in memory, increasing them might increase memory use`,
			Name:      "chunk_size",
			Required:  true,
			Exclusive: true,
			Advanced:  true,
			Default:   512,
			Examples: []fs.OptionExample{
				{Value: "512", Help: "512 KB (Fastest, heavy load)"},
				{Value: "256", Help: "256 KB (Faster, high load)"},
				{Value: "128", Help: "128 KB (Fast, chunky load)"},
				{Value: "64", Help: "64 KB (Slow, tiny load)"},
				{Value: "32", Help: "32 KB (Slower, light load)"},
				{Value: "16", Help: "16 KB (Slowest, lightest load "},
			},
		},
	}
)
