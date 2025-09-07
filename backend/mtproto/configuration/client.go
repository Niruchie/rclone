package configuration

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/rclone/rclone/backend/mtproto/configuration/logging"
	"github.com/rclone/rclone/backend/mtproto/configuration/options"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/pacer"
)

// MTProtoService with all its properties.
type MTProtoService struct {
	datacenter      *telegram.NearestDc
	client          *telegram.Client
	config          *telegram.Config
	appConfig       *telegram.HelpAppConfigObj
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
		return nil, logging.ErrInvalidBase64PublicKey
	}

	// ? Decode the PEM block.
	block, _ := pem.Decode([]byte(decoded))
	key, err := x509.ParsePKCS1PublicKey(block.Bytes)
	if err != nil {
		return nil, logging.ErrInvalidRSAPublicKey
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
		Cache: telegram.NewCache(
			telegram.Restricted,
			&telegram.CacheConfig{
				Disabled: !options.DisableCache,
				Memory:   options.DisableCache,
				LogName:  telegram.Restricted,
				LogLevel: telegram.LogDisable,
				MaxSize:  math.MaxInt16,
			},
		),
		FloodHandler:  mtproto.handleFloodWait,
		MemorySession: options.MemorySession,
		LogLevel:      telegram.LogDisable,
		TestMode:      service.TestServer,
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

		go mtproto.handleUpdates()
		_ = mtproto.UpdateConfig()
	}

	mtproto.client = client
	return mtproto, nil
}

// Waits for the specified duration if a flood wait error is encountered.
//
// Parameters:
//
//	err error - The error to check for flood wait.
//
// Returns:
//
//	bool - Returns true if a flood wait was handled, otherwise false.
func (mtproto *MTProtoService) handleFloodWait(err error) bool {
	if wait := telegram.GetFloodWait(err); wait > 0 {
		log := "flood wait, pausing for %d seconds"
		fs.Infof(logging.LoggerString(err), log, wait)
		time.Sleep(time.Duration(wait) * time.Second)
		return true
	}
	return false
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
//
//	u telegram.Update - The update received from the MTProto API.
//	_ *telegram.Client - The client instance (unused).
//
// Returns:
//
//	error - Returns an error if updating the config fails, otherwise nil.
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

// Returns the pacer instance from the filesystem.
//
// The pacer is used to avoid rate limiting from data centers.
func (mtproto *MTProtoService) Pacer() *pacer.Pacer {
	return mtproto.pacer
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

// ----- Channel Management -----

// Create a new supergroup with forum topics.
//
// Parameters:
//
//	_ context.Context - The context for the request.
//	title string - The title of the channel.
//
// Returns:
//
//	channel telegram.Channel - The created channel.
//	created bool - Whether the channel was created successfully.
//	err error - If an error occurs while creating the channel.
func (mtproto *MTProtoService) CreateChannel(_ context.Context, title string) (channel telegram.Channel, created bool, err error) {
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

// ----- Forum Topic Management -----

// Fetch [forum topics] (directories) from the client.
//
// Parameters:
//
//	_ context.Context - The context for the request.
//	forumTopicSearch telegram.ForumTopicObj - The search criteria for the forum topics.
//
// [forum topics]: https://core.telegram.org/api/forum#forum-topics
func (mtproto *MTProtoService) GetTopics(_ context.Context, forumTopicSearch telegram.ForumTopicObj) (forumTopics []telegram.ForumTopicObj, err error) {
	client, err := mtproto.Client()
	if err != nil {
		return forumTopics, err
	}

	channel, err := client.GetChannel(mtproto.SupergroupId)
	if err != nil {
		return forumTopics, err
	}

	input := &telegram.InputChannelObj{
		AccessHash: channel.AccessHash,
		ChannelID:  channel.ID,
	}

	forum, err := client.ChannelsGetForumTopics(&telegram.ChannelsGetForumTopicsParams{
		Q:       forumTopicSearch.Title,
		Limit:   math.MaxInt32,
		Channel: input,
	})
	if err != nil {
		return forumTopics, err
	}

	forumTopics = []telegram.ForumTopicObj{}
	for i := range forum.Topics {
		if topic, ok := forum.Topics[i].(*telegram.ForumTopicObj); ok {
			forumTopics = append(forumTopics, *topic)
		}
	}

	return forumTopics, nil
}

// Create a new [forum topic] (directory) within the [forum supergroup].
//
// Parameters:
//
//	_ context.Context - The context for the request.
//	title string - The title of the topic.
//
// Returns:
//
//	forumTopic telegram.ForumTopicObj - The created forum topic.
//	created bool - Whether the forum topic was created successfully.
//	err error - If an error occurs while creating the forum topic.
//
// [forum topic]: https://core.telegram.org/api/forum#forum-topics
// [forum supergroup]: https://core.telegram.org/api/channel#forums
func (mtproto *MTProtoService) CreateTopic(ctx context.Context, forumTopicIn telegram.ForumTopicObj) (forumTopic telegram.ForumTopicObj, created bool, err error) {
	mtproto.lockDirectories.Lock()
	defer mtproto.lockDirectories.Unlock()

	// Search for existing forum topic with request.
	topics, err := mtproto.GetTopics(ctx, forumTopicIn)
	if err != nil {
		return forumTopic, created, err
	}

	// Search within the filtered forum topics.
	for _, topic := range topics {
		if topic.Title == forumTopicIn.Title {
			forumTopic = topic
			return forumTopic, created, nil
		}
	}

	// Create the forum topic if it does not exist
	client, err := mtproto.Client()
	if err != nil {
		return forumTopic, created, err
	}

	channel, err := client.GetChannel(mtproto.SupergroupId)
	if err != nil {
		return forumTopic, created, err
	}

	input := &telegram.InputChannelObj{
		AccessHash: channel.AccessHash,
		ChannelID:  channel.ID,
	}

	response, err := client.ChannelsCreateForumTopic(&telegram.ChannelsCreateForumTopicParams{
		Title:    forumTopicIn.Title,
		RandomID: rand.Int63(),
		Channel:  input,
	})

	updates, ok := response.(*telegram.UpdatesObj)
	if !ok || len(updates.Updates) <= 0 {
		return forumTopic, created, logging.ErrOperationWithoutUpdates
	}

	forumTopicIds := []int32{}
	switch u := updates.Updates[0].(type) {
	// The service message update.
	case *telegram.UpdateMessageID:
		forumTopicIds = append(forumTopicIds, u.ID)

	// Search for the service message, on message update.
	case *telegram.UpdateNewChannelMessage:
		if message, ok := u.Message.(*telegram.MessageService); ok {
			forumTopicIds = append(forumTopicIds, message.ID)
			break
		}
		return forumTopic, created, logging.ErrOperationWithoutUpdates

	default:
		return forumTopic, created, logging.ErrOperationWithoutUpdates
	}

	// Fetch the created forum topic using its message id.
	query, err := client.ChannelsGetForumTopicsByID(input, forumTopicIds)
	switch {
	case err != nil:
		return forumTopic, created, err
	case len(query.Topics) <= 0:
		return forumTopic, created, logging.ErrOperationWithoutUpdates
	}

	createdForumTopic, ok := query.Topics[0].(*telegram.ForumTopicObj)
	if !ok {
		return forumTopic, created, logging.ErrOperationWithoutUpdates
	}

	created = true
	forumTopic = *createdForumTopic
	return forumTopic, created, err
}

// Remove the history of a [forum topic] (directory) within the [forum supergroup].
//
// Check the [forum topic] (directory) is empty before calling this method.
//
// The [forum topic] will be deleted along with its history.
//
// Parameters:
//
//	_ context.Context - The context for the request.
//	forumTopicIn telegram.ForumTopicObj - The forum topic to delete.
//
// [forum topic]: https://core.telegram.org/api/forum#forum-topics
// [forum supergroup]: https://core.telegram.org/api/channel#forums
func (mtproto *MTProtoService) DeleteTopic(ctx context.Context, forumTopicIn telegram.ForumTopicObj) (err error) {
	mtproto.lockDirectories.Lock()
	defer mtproto.lockDirectories.Unlock()

	// Some forum topics cannot be deleted.
	switch forumTopicIn.ID {
	case 0x00:
		return logging.ErrUnsupportedOperation
	case options.ChannelRootTopicId:
		return logging.ErrUnsupportedOperation
	default:
		client, err := mtproto.Client()
		if err != nil {
			return err
		}

		channel, err := client.GetChannel(mtproto.SupergroupId)
		if err != nil {
			return err
		}

		input := &telegram.InputChannelObj{
			AccessHash: channel.AccessHash,
			ChannelID:  channel.ID,
		}

		_, err = client.ChannelsDeleteTopicHistory(input, forumTopicIn.ID)
		return err
	}
}

// Update the [forum topic] (directory) within the [forum supergroup].
//
// Parameters:
//
//	_ context.Context - The context for the request.
//	forumTopicIn telegram.ForumTopicObj - The forum topic to update.
//
// Returns:
//
//	forumTopic telegram.ForumTopicObj - The updated forum topic.
//	updated bool - Whether the forum topic was updated successfully.
//	err error - If an error occurs while updating the forum topic.
//
// [forum topic]: https://core.telegram.org/api/forum#forum-topics
// [forum supergroup]: https://core.telegram.org/api/channel#forums
func (mtproto *MTProtoService) UpdateTopic(_ context.Context, forumTopicIn telegram.ForumTopicObj) (forumTopic telegram.ForumTopicObj, updated bool, err error) {
	mtproto.lockDirectories.Lock()
	defer mtproto.lockDirectories.Unlock()

	// Some forum topics should not be updated.
	switch forumTopicIn.ID {
	case 0x00:
		return forumTopic, updated, logging.ErrUnsupportedOperation
	case options.ChannelRootTopicId:
		return forumTopic, updated, logging.ErrUnsupportedOperation
	default:
		client, err := mtproto.Client()
		if err != nil {
			return forumTopic, updated, err
		}

		channel, err := client.GetChannel(mtproto.SupergroupId)
		if err != nil {
			return forumTopic, updated, err
		}

		input := &telegram.InputChannelObj{
			AccessHash: channel.AccessHash,
			ChannelID:  channel.ID,
		}

		response, err := client.ChannelsEditForumTopic(&telegram.ChannelsEditForumTopicParams{
			Title:   forumTopicIn.Title,
			Channel: input,
			TopicID: forumTopicIn.ID,
		})
		if err != nil {
			return forumTopic, updated, err
		}

		updates, ok := response.(*telegram.UpdatesObj)
		if !ok || len(updates.Updates) <= 0 {
			return forumTopic, updated, logging.ErrOperationWithoutUpdates
		}

		switch u := updates.Updates[0].(type) {
		// The service message update.
		case *telegram.UpdateMessageID:
			updated = true
			return forumTopicIn, updated, nil

		// Search for the service message, on message update.
		case *telegram.UpdateNewChannelMessage:
			if _, ok := u.Message.(*telegram.MessageService); ok {
				updated = true
				return forumTopicIn, updated, nil
			}
			return forumTopic, updated, logging.ErrOperationWithoutUpdates
		default:
			return forumTopic, updated, logging.ErrOperationWithoutUpdates
		}
	}
}
