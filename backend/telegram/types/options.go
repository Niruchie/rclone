package types

import (
	"strconv"

	"github.com/rclone/rclone/fs"
)

// Options defines the configuration for this backend
type Options struct {
	AppId       int32  `config:"app_id"`
	AppHash     string `config:"app_hash"`
	BotToken    string `config:"bot_token"`
	PublicKey   string `config:"public_key"`
	ChannelId   int64  `config:"channel_id"`
	PhoneNumber string `config:"phone_number"`

	ChunkSize             int64  `config:"chunk_size"`
	MaxRetries            int    `config:"max_retries"`
	TestServer            bool   `config:"test_server"`
	StringSession         string `config:"string_session"`
	MaxCacheTime          int    `config:"max_cache_time"`
	MaxConnections        int    `config:"max_connections"`
	MaxObjectSizeAccepted int64  `config:"max_object_size_accepted"`
}

// Constants to be used in the backend.
var (
	// The root topic ID for the filesystem.
	//  - The root topic ID is the first topic created.
	//  - The root topic ID must not be deleted.
	//  - The root topic ID is 1.
	ChannelRootTopicId int32 = 1

	// Session to use when login is performed.
	// Also used to login to Telegram Bot API.
	//  - Generated after rclone configuration.
	//  - An empty string for the session.
	SessionStringEmpty string = ""

	// [Telegram API | Flood Wait] HTTP Status Code for RPC errors.
	//  - These [Telegram API | Transport Errors] are known to be
	//  - Telegram API returns 420 when the request is throttled.
	//
	// [Telegram API | Flood Wait]: https://core.telegram.org/api/errors
	// [Telegram API | Transport Errors]: https://core.telegram.org/mtproto/mtproto-transports#transport-errors
	StatusTelegramFloodWait int64 = 420

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

	// The maximum object size accepted by the backend.
	//  - Default is 2 GiB as we are using MTProto.
	//  - [Local server] lets upload 2 GiB files.
	//  - Files are uploaded with bots.
	//
	// [Local server]: https://core.telegram.org/bots/api#using-a-local-bot-api-server
	MaxObjectSizeAccepted int64 = 2 << 30

	// When using Streamed Downloads, use precise size for the download part size.
	// The request will download the exact size and assing it to the reader.
	// Non-buffered bytes will be cached in the reader for next read.
	MaxDownloadPreciseSize int32 = 1 << 20

	// Contains the configuration options for the backend.
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
		},

		{
			Help:      "App Hash for Telegram API",
			Name:      "app_hash",
			Advanced:  false,
			Required:  true,
			Sensitive: true,
		},

		{
			Help:      "Bot Token for Telegram API",
			Name:      "bot_token",
			Advanced:  false,
			Required:  true,
			Sensitive: true,
		},

		{
			Help:      "Public Key for Telegram API (Should be base64 encoded or empty, PEM format)",
			Name:      "public_key",
			Advanced:  false,
			Required:  true,
			Sensitive: true,
			Default:   "",
		},

		{
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
		},

		{
			Help:     "Maximum number of connections to use. Can lead to flood rate limiting if too high",
			Name:     "max_connections",
			Provider: "telegram",
			Required: true,
			Advanced: true,
			Default:  10,
		},

		{
			Help:     "Maximum number of retries to perform on errors",
			Name:     "max_retries",
			Provider: "telegram",
			Required: true,
			Advanced: true,
			Default:  5,
		},

		{
			Help:     "Maximum time in seconds for the cached resources between updates",
			Name:     "max_cache_time",
			Provider: "telegram",
			Required: true,
			Advanced: true,
			Default:  30,
		},

		{
			Help:      `The maximum object size accepted by the backend. Note that this is the maximum size of the object that can be uploaded to the Telegram servers`,
			Name:      "max_object_size_accepted",
			Provider:  "telegram",
			Required:  true,
			Exclusive: true,
			Advanced:  true,
			Default:   2 << 30,
			Examples: []fs.OptionExample{
				{Value: strconv.Itoa(2 << 30), Help: "2 GiB (Default, non-premium)"},
				{Value: strconv.Itoa(4 << 30), Help: "4 GiB (To be confirmed)"},
			},
		},

		// The part size for the multipart upload.
		//   - Default and maximum size is 512 KB.
		//   - The part size must be divisible by 1KB.
		//   - https://core.telegram.org/api/files#uploading-files
		{
			Help:      `Files will be uploaded in chunks this size. Note that these chunks might be buffered in memory, increasing them might increase memory use`,
			Name:      "chunk_size",
			Provider:  "telegram",
			Required:  true,
			Exclusive: true,
			Advanced:  true,
			Default:   512 << 10,
			Examples: []fs.OptionExample{
				{Value: strconv.Itoa(512 << 10), Help: "512 KB (Fastest, heavy load)"},
				{Value: strconv.Itoa(256 << 10), Help: "256 KB (Faster, high load)"},
				{Value: strconv.Itoa(128 << 10), Help: "128 KB (Fast, chunky load)"},
				{Value: strconv.Itoa(64 << 10), Help: "64 KB (Slow, tiny load)"},
				{Value: strconv.Itoa(32 << 10), Help: "32 KB (Slower, light load)"},
				{Value: strconv.Itoa(16 << 10), Help: "16 KB (Slowest, lightest load)"},
			},
		},
	}
)
