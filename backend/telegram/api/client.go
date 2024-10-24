package api

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"sync"
	"time"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/rclone/rclone/backend/telegram/types"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/cache"
	"github.com/rclone/rclone/lib/pacer"
)

type TelegramClient struct {
	lockDirectories sync.Mutex
	lockFiles       sync.Mutex
	mtproto  *telegram.Client
	bot      *telegram.Client
	channels *cache.Cache
	topics   *cache.Cache
	pacer    *fs.Pacer
	types.Options
}

// Decode the public key from the client obtained from the [Telegram Apps].
//
// Definition:
//
//  DecodePublicKeys() ([]*rsa.PublicKey, error)
//
// Returns:
//
//  []*rsa.PublicKey - The decoded public key.
//  error - If an error occurs while decoding the public key.
//
// [Telegram Apps]: https://core.telegram.org/apps
func (tc *TelegramClient) DecodePublicKeys() ([]*rsa.PublicKey, error) {
	// ? Decode the public key.
	decoded, err := base64.StdEncoding.DecodeString(tc.PublicKey)
	if err != nil {
		return nil, types.ErrInvalidBase64PublicKey
	}

	// ? Decode the PEM block.
	block, _ := pem.Decode([]byte(decoded))
	key, err := x509.ParsePKCS1PublicKey(block.Bytes)
	if err != nil {
		return nil, types.ErrInvalidRSAPublicKey
	}

	return []*rsa.PublicKey{key}, nil
}

// Get the client from the [Telegram MTProto API] and use with the [MTProto API Methods].
//
// Definition:
//
//  ConnectMTProto(openSession bool) (*telegram.Client, error)
//
// Parameters:
//
//  openSession bool - Whether the session should open with SessionString.
//
// Returns:
//
//  *telegram.Client - The Telegram MTProto API client.
//  error - If an error occurs while connecting to the Telegram MTProto API.
//
// [Telegram MTProto API]: https://core.telegram.org/mtproto
// [MTProto API Methods]: https://core.telegram.org/methods
func (tc *TelegramClient) ConnectMTProto(openSession bool) (*telegram.Client, error) {
	var session string = types.SessionStringEmpty
	if openSession {
		session = tc.StringSession
	}

	// ? From current client get the public keys.
	keys, err := tc.DecodePublicKeys()
	if err != nil {
		return nil, err
	}

	// ? The App ID and App Hash are used to authenticate the client.
	client, err := telegram.NewClient(telegram.ClientConfig{
		DeviceConfig: telegram.DeviceConfig{
			DeviceModel:   fmt.Sprintf("rclone %s %s", fs.VersionTag, fs.VersionSuffix),
			SystemVersion: fs.VersionSuffix,
			AppVersion:    fs.VersionTag,
		},
		LogLevel:      telegram.LogError,
		TestMode:      tc.TestServer,
		MemorySession: true,
		DisableCache:  true,
		LangCode:      "en",

		AppHash:       tc.AppHash,
		AppID:         tc.AppId,
		StringSession: session,
		PublicKeys:    keys,
	})

	if err != nil {
		fs.Error(types.LoggerString(tc), err.Error())
		return nil, types.ErrInvalidClient
	}

	// ? Connect the client to the Telegram MTProto API.
	err = client.Connect()
	if err != nil {
		fs.Error(types.LoggerString(tc), err.Error())
		return nil, types.ErrInvalidClientCouldNotConnect
	}

	return client, err
}

// Get the client from the [Telegram Bot API].
//
// Definition:
//
//  ConnectBot() (*telegram.Client, error)
//
// Returns:
//
//  *telegram.Client - The Telegram Bot API client.
//  error - If an error occurs while connecting to the Telegram Bot API.
//
// [Telegram Bot API]: https://core.telegram.org/bots/api
func (tc *TelegramClient) ConnectBot() (*telegram.Client, error) {
	// ? Get an MTProto client with empty string session.
	client, err := tc.ConnectMTProto(false)
	if err != nil {
		fs.Error(types.LoggerString(tc), err.Error())
		return nil, err
	}

	// ? Connect the client to the Telegram Bot API.
	err = client.LoginBot(tc.BotToken)
	if err != nil {
		fs.Error(types.LoggerString(tc), err.Error())
		return nil, types.ErrInvalidClientCouldNotConnectBot
	}

	return client, err
}

// Try to reconnect the Telegram MTProto and Bot instances.
// If using a Telegram Test Data Center, simulate the bot with the MTProto.
//
// Definition:
//
//  ActiveReconnect() error
//
// Returns:
//
//  error - If an error occurs while reconnecting.
func (tc *TelegramClient) ActiveReconnect() error {
	if !tc.mtproto.TcpActive() {
		err := tc.mtproto.Reconnect(true)
		if err != nil {
			fs.Error(types.LoggerString(tc.mtproto), err.Error())
			return err
		}
	}

	switch tc.TestServer {
	case true:
		// ? Already connected simulation with MTProto.
		tc.bot = tc.mtproto
		return nil
	case false:
		// ? Reconnect the bot if it's not active.
		if !tc.bot.TcpActive() {
			err := tc.bot.Reconnect(true)
			if err != nil {
				fs.Error(types.LoggerString(tc.bot), err.Error())
				return err
			}
		}
	}

	return nil
}

// Connect the filesystem client to the Telegram API.
//   - The client would connect to the Telegram API using the MTProto and Bot API.
//   - While using the test server, MTProto would be used for the Telegram Bot API.
//   - Also a session uses [fs.pacer] to avoid rate limiting by [Flood Wait] from data centers.
//   - Almost all methods of Bot API are available through MTProto, not the same for the reverse.
//
// Definition:
//
//  Connect(ctx context.Context) (*telegram.Client, *telegram.Client, error)
//
// Returns:
//
//  *telegram.Client - The Telegram MTProto API client.
//  *telegram.Client - The Telegram Bot API client.
//  error - If an error occurs while connecting to the Telegram API.
//
// [fs.pacer]: https://pkg.go.dev/github.com/rclone/rclone/lib/pacer
// [Flood Wait]: https://core.telegram.org/api/errors#420-flood
func (tc *TelegramClient) Connect(ctx context.Context) (*telegram.Client, *telegram.Client, error) {
	tc.lockDirectories = sync.Mutex{}
	tc.lockFiles = sync.Mutex{}
	tc.channels = cache.New()
	tc.topics = cache.New()

	maxCacheDuration := time.Duration(tc.MaxCacheTime) * time.Second
	tc.channels.SetExpireDuration(maxCacheDuration)
	tc.topics.SetExpireDuration(maxCacheDuration)

	tc.pacer = fs.NewPacer(ctx, pacer.NewDefault())
	tc.pacer.SetMaxConnections(tc.MaxConnections)
	tc.pacer.SetRetries(tc.MaxRetries)

	var mtproto *telegram.Client
	var bot *telegram.Client
	var err error

	switch tc.TestServer {
	case true:
		fs.Log(types.LoggerString(tc.mtproto), "Connecting to the test server, might take a while more than usual.")
		fs.Log(types.LoggerString(tc.mtproto), "Simulating the bot with the MTProto.")

		mtproto, err = tc.ConnectMTProto(true)
		if err != nil {
			fs.Error(types.LoggerString(tc.mtproto), err.Error())
			return nil, nil, err
		}

		bot = mtproto
	default:
		mtproto, err = tc.ConnectMTProto(true)
		if err != nil {
			fs.Error(types.LoggerString(tc.mtproto), err.Error())
			return nil, nil, err
		}

		bot, err = tc.ConnectBot()
		if err != nil {
			fs.Error(types.LoggerString(tc.bot), err.Error())
			return nil, nil, err
		}
	}

	tc.mtproto = mtproto
	tc.bot = bot
	return tc.mtproto, tc.bot, nil
}

// Disconnect the filesystem client from the Telegram API.
//
// Definition:
//
//  Disconnect()
//
// The client would disconnect from the Telegram MTProto and Bot API.
func (tc *TelegramClient) Disconnect() {
	tc.mtproto.Disconnect()
	tc.bot.Disconnect()
}

// Authorize the filesystem client with the Telegram API.
func (tc *TelegramClient) Authorize() (*TelegramClient, error) {
	mtproto, err := tc.ConnectMTProto(false)
	if err != nil {
		fs.Error(types.LoggerString(tc), err.Error())
		return nil, err
	}

	// ? Sign in with the code.
	_, err = mtproto.Login(tc.PhoneNumber, &telegram.LoginOptions{})
	if err != nil {
		fs.Error(types.LoggerString(mtproto), err.Error())
		return nil, err
	}

	tc.mtproto = mtproto
	tc.bot = mtproto
	return tc, nil
}

// Returns the Telegram MTProto instance from the filesystem.
//
// Definition:
//
//  MTProto() *telegram.Client
//
// The MTProto would try to reconnect if it's not active.
// If an error occurs while reconnecting, it returns nil.
func (tc *TelegramClient) MTProto() (*telegram.Client, error) {
	err := tc.ActiveReconnect()
	if err != nil {
		fs.Error(types.LoggerString(tc), err.Error())
		return nil, err
	}

	return tc.mtproto, nil
}

// Returns the pacer instance from the filesystem.
//
// Definition:
//
//  Pacer() *fs.Pacer
//
// The pacer is used to avoid rate limiting from data centers.
func (tc *TelegramClient) Pacer() *fs.Pacer {
	return tc.pacer
}

// Returns the Telegram Bot instance from the filesystem.
//
// Definition:
//
//  Bot() *telegram.Client
//
// The bot would try to reconnect if it's not active.
// If an error occurs while reconnecting, it returns nil.
func (tc *TelegramClient) Bot() (*telegram.Client, error) {
	err := tc.ActiveReconnect()
	if err != nil {
		fs.Error(types.LoggerString(tc), err.Error())
		return nil, err
	}

	return tc.bot, nil
}
