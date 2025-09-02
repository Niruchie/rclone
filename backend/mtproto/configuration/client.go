package configuration

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"sync"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/rclone/rclone/backend/mtproto/configuration/logging"
	"github.com/rclone/rclone/backend/mtproto/configuration/options"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/cache"
	"github.com/rclone/rclone/lib/pacer"
)

// MTProtoService with all its properties.
type MTProtoService struct {
	datacenter      *telegram.NearestDc
	client          *telegram.Client
	config          *telegram.Config
	appConfig       *telegram.HelpAppConfigObj
	channels        *cache.Cache
	pacer           *pacer.Pacer
	lockDirectories sync.Mutex
	options.Options
}

// NewMTProtoService creates a new MTProtoService instance.
func NewMTProtoService(ctx context.Context) *MTProtoService {
	pacer := pacer.New()
	
	service := &MTProtoService{
		appConfig:       &telegram.HelpAppConfigObj{},
		config:          &telegram.Config{},
		lockDirectories: sync.Mutex{},
		channels:        cache.New(),
		pacer:           pacer,
		client:          nil,
	}

	// TODO: The values for pacer are set to 0x00.
	pacer.SetMaxConnections(service.MaxConnections)
	pacer.SetRetries(service.MaxRetries)
	return service
}

// Decode the public key from the client obtained from the [Telegram Apps].
//
// Definition:
//
//	DecodePublicKeys() ([]*rsa.PublicKey, error)
//
// Returns:
//
//	[]*rsa.PublicKey - The decoded public key.
//	error - If an error occurs while decoding the public key.
//
// [Telegram Apps]: https://core.telegram.org/apps
func (mtproto *MTProtoService) DecodePublicKeys() ([]*rsa.PublicKey, error) {
	// ? Decode the public key.
	decoded, err := base64.StdEncoding.DecodeString(mtproto.PublicKey)
	if err != nil {
		// return nil, types.ErrInvalidBase64PublicKey
	}

	// ? Decode the PEM block.
	block, _ := pem.Decode([]byte(decoded))
	key, err := x509.ParsePKCS1PublicKey(block.Bytes)
	if err != nil {
		// return nil, types.ErrInvalidRSAPublicKey
	}

	return []*rsa.PublicKey{key}, nil
}

// Get the client from the [MTProto API] and use with the [MTProto API Methods].
//
// Definition:
//
//	ServiceConnect(openSession bool) (*telegram.Client, error)
//
// Parameters:
//
//	openSession bool - Whether the session should open with SessionString.
//
// Returns:
//
//	*telegram.Client - The MTProto API client.
//	error - If an error occurs while connecting to the MTProto API.
//
// [MTProto API]: https://core.telegram.org/mtproto
// [MTProto API Methods]: https://core.telegram.org/methods
func (mtproto *MTProtoService) ServiceConnect(openSession bool) (*telegram.Client, error) {
	var service options.Options = mtproto.Options
	var session string = options.SessionStringEmpty
	if openSession {
		session = mtproto.StringSession
	}

	// ? From current client get the public keys.
	keys, err := mtproto.DecodePublicKeys()
	if err != nil {
		return nil, err
	}

	// ? The App ID and App Hash are used to authenticate the client.
	client, err := telegram.NewClient(telegram.ClientConfig{
		DeviceConfig: telegram.DeviceConfig{
			DeviceModel:   options.DefaultDeviceModel,
			LangCode:      options.DefaultLangCode,
			AppVersion:    fs.VersionSuffix,
			SystemVersion: fs.VersionTag,
		},
		MemorySession: options.MemorySession,
		DisableCache:  options.DisableCache,
		TestMode:      service.TestServer,
		LogLevel:      telegram.LogError,
		AppHash:       service.AppHash,
		AppID:         service.AppId,
		StringSession: session,
		PublicKeys:    keys,
	})

	if err != nil {
		fs.Error(logging.LoggerString(mtproto), err.Error())
		return nil, logging.ErrInvalidClient
	}

	// Connect the client to the MTProto API.
	err = client.Connect()
	if err != nil {
		fs.Error(logging.LoggerString(mtproto), err.Error())
		return nil, logging.ErrInvalidClientCouldNotConnect
	}

	return client, err
}

// Authorize the MTProto API client with the found credential options.
//
// Returns:
//
//	*MTProtoService - The authorized MTProto API service.
//	error - If an error occurs while authorizing the client.
func (mtproto *MTProtoService) Authorize() (*MTProtoService, error) {
	var client *telegram.Client = mtproto.client
	var err error = logging.ErrInvalidClient

	if client != nil {
		mtproto.ActiveReconnect()
		return mtproto, nil
	}

	switch mtproto.StringSession {
	case "":
		client, err = mtproto.ServiceConnect(false)
		if err != nil {
			fs.Error(logging.LoggerString(mtproto), err.Error())
			return nil, err
		}
		// ? Sign in with the code.
		_, err = client.Login(mtproto.PhoneNumber, &telegram.LoginOptions{})
		if err != nil {
			fs.Error(logging.LoggerString(client), err.Error())
			return nil, err
		}
	default:
		client, err = mtproto.ServiceConnect(true)
		if err != nil {
			fs.Error(logging.LoggerString(mtproto), err.Error())
			return nil, err
		}
	}

	mtproto.client = client
	go mtproto.handleUpdates()
	_ = mtproto.UpdateConfig()
	return mtproto, nil
}

// Listens for updates from the MTProto API and delegates them for further processing.
//
// Called only after the client has been authorized.
func (mtproto *MTProtoService) handleUpdates() {
	if client, err := mtproto.Client(); err == nil {
		client.On(telegram.OnRaw, mtproto.handlerOnRawUpdate)
	}
}

// handlerOnRawUpdate processes raw updates received from the client.
//
// It handles configuration updates and logs other raw updates for debugging purposes.
//
// Parameters:
//   u telegram.Update - The update received from the MTProto API.
//   _ *telegram.Client - The client instance (unused).
//
// Returns:
//   error - Returns an error if updating the config fails, otherwise nil.
func (mtproto *MTProtoService) handlerOnRawUpdate(u telegram.Update, _ *telegram.Client) error {
	switch update := u.(type) {
	case *telegram.UpdateConfig:
		err := mtproto.UpdateConfig()
		if err != nil {
			log := logging.LoggerString(update)
			fs.Error(log, err.Error())
		}
	default:
		fs.Debugf(
			logging.LoggerString(update),
			"Received raw update: %v",
			update,
		)
	}

	return nil
}

// Try to reconnect the MTProto instance.
// If using a Test Data Center, also reconnect to MTProto API.
//
// Returns:
//
//	error - If an error occurs while reconnecting.
func (mtproto *MTProtoService) ActiveReconnect() error {
	var client *telegram.Client = mtproto.client
	if client == nil {
		err := logging.ErrInvalidClient
		fs.Error(logging.LoggerString(mtproto), err.Error())
		return err
	}

	tcp := client.TcpState()
	active := tcp.Active.Load()
	if !active {
		err := client.Reconnect(true)
		if err != nil {
			fs.Error(logging.LoggerString(mtproto), err.Error())
			return err
		}
	}

	return nil
}

// Returns the MTProto Client instance from the service.
//
// The client would try to reconnect if it's not active.
// If an error occurs while reconnecting, it returns nil.
func (mtproto *MTProtoService) Client() (*telegram.Client, error) {
	err := mtproto.ActiveReconnect()
	if err != nil {
		fs.Error(logging.LoggerString(mtproto), err.Error())
		return nil, err
	}

	return mtproto.client, nil
}

// Follow the requirements for updating the MTProto configuration.
// See [Client Configuration].
//
// [Client Configuration]: https://core.telegram.org/api/config#client-configuration
func (mtproto *MTProtoService) UpdateConfig() error {
	client, err := mtproto.Client()
	if err != nil {
		return err
	}

	dc, err := client.HelpGetNearestDc()
	switch err {
	case nil:
		mtproto.datacenter = dc
	default:
		return err
	}

	appConfig, _ := client.HelpGetAppConfig(mtproto.appConfig.Hash)
	switch cfg := appConfig.(type) {
	case *telegram.HelpAppConfigObj:
		mtproto.appConfig = cfg
	}

	config, err := client.HelpGetConfig()
	switch err {
	case nil:
		mtproto.config = config
	default:
		return err
	}

	return nil
}

// Create a new supergroup with forum topics.
//
// Parameters:
//
//	ctx context.Context - The context for the request.
//	title string - The title of the channel.
//
// Returns:
//
//	channel telegram.Channel - The created channel.
//	created bool - Whether the channel was created successfully.
//	err error - If an error occurs while creating the channel.
func (mtproto *MTProtoService) CreateChannel(ctx context.Context, title string) (channel telegram.Channel, created bool, err error) {
	mtproto.lockDirectories.Lock()
	defer mtproto.lockDirectories.Unlock()

	client, err := mtproto.Client()
	if err != nil {
		return channel, false, err
	}

	// Supergroup non importable with forum topics
	details := &telegram.ChannelsCreateChannelParams{
		About:     title,
		Title:     title,
		ForImport: false,
		Megagroup: true,
		Forum:     true,
	}

	raw, err := client.ChannelsCreateChannel(details)
	if err != nil {
		return channel, false, err
	}

	updates, ok := raw.(*telegram.UpdatesObj)
	if !ok || len(updates.Chats) <= 0 {
		return channel, false, err
	}

	update, created := updates.Chats[0].(*telegram.Channel)
	if created {
		channel = *update
	}

	return channel, created, err
}
