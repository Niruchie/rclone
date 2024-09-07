package api

import (
	"context"
	"math"
	"math/rand"

	"github.com/amarnathcjd/gogram"
	"github.com/amarnathcjd/gogram/telegram"
	"github.com/pkg/errors"
	"github.com/rclone/rclone/backend/telegram/types"
	"github.com/rclone/rclone/fs"
)

type SearchMessagesTopicReturn struct {
	Messages   []telegram.Message
	Amount     int64
	Incomplete bool
	Offset     int32
	Error      error
}

func (tc *TelegramClient) CreateTopic(ctx context.Context, title string) (*telegram.ForumTopicObj, bool, error) {
	defer tc.lockDirectories.Unlock()
	tc.lockDirectories.Lock()
	var err error

	// ? Check whether the topic already exists.
	topics, err := tc.GetTopics(ctx, title)
	if err != nil {
		return nil, false, err
	}

	for _, topic := range topics {
		if topic.Title == title {
			return topic, false, nil
		}
	}

	// ? Create the topic if it does not exist.
	var topic *telegram.ForumTopicObj = nil

	err = tc.pacer.Call(func() (bool, error) {
		mtproto, err := tc.MTProto()
		if err != nil {
			return false, err
		}

		channel, err := tc.GetChannel(ctx)
		if err != nil {
			return false, err
		}

		updates, err := mtproto.ChannelsCreateForumTopic(&telegram.ChannelsCreateForumTopicParams{
			Channel: &telegram.InputChannelObj{
				AccessHash: channel.AccessHash,
				ChannelID:  channel.ID,
			},
			RandomID: rand.Int63(),
			Title:    title,
		})

		if cause, ok := errors.Cause(err).(*gogram.ErrResponseCode); ok {
			// ? Check if the error is a flood wait.
			if cause.Code == 420 {
				fs.LogPrint(fs.LogLevelWarning, err.Error())
				return true, cause
			}
		}

		// ? Read all the updates and message from service, creating a new topic.
		// * Telegram creates a service message on channel,
		// * That message is replied to write on the topic.
		if updates, ok := updates.(*telegram.UpdatesObj); ok {
			for _, update := range updates.Updates {
				if message, ok := update.(*telegram.UpdateNewChannelMessage); ok {
					if serviceMsg, ok := message.Message.(*telegram.MessageService); ok {
						topic = &telegram.ForumTopicObj{
							FromID:         serviceMsg.PeerID,
							Date:           serviceMsg.Date,
							ID:             serviceMsg.ID,
							ReadInboxMaxID: serviceMsg.ID,
							TopMessage:     serviceMsg.ID,
							Title:          title,
							My:             true,
						}

						break
					}
				}
			}
		}

		return false, err
	})

	if err != nil {
		return nil, false, err
	}

	return topic, topic != nil, err
}

// Returns the channel for the filesystem.
//   - The channel is cached for a certain period of time.
//   - The request is paced to avoid flooding the server.
//   - If an error occurs on fetch, error is returned.
//
// Definition:
//
//	GetChannel(ctx context.Context) (*telegram.Channel, error)
//
// Returns:
//
//	*telegram.Channel - The channel for the filesystem.
//	error - If an error occurs while getting the channel.
func (tc *TelegramClient) GetChannel(ctx context.Context) (*telegram.Channel, error) {
	cache, err := tc.channels.Get("mtproto", func(key string) (interface{}, bool, error) {
		var channel *telegram.Channel = nil

		err := tc.pacer.Call(func() (bool, error) {
			mtproto, err := tc.MTProto()
			if err != nil {
				return false, err
			}

			response, err := mtproto.ChannelsGetChannels([]telegram.InputChannel{
				&telegram.InputChannelObj{
					ChannelID: tc.ChannelId,
				},
			})

			if cause, ok := errors.Cause(err).(*gogram.ErrResponseCode); ok {
				// ? Check if the error is a flood wait.
				if cause.Code == 420 {
					fs.LogPrint(fs.LogLevelWarning, err.Error())
					return true, cause
				}
			}

			if messagesChats, ok := response.(*telegram.MessagesChatsObj); ok {
				for _, chat := range messagesChats.Chats {
					if single, ok := chat.(*telegram.Channel); ok {
						if single.ID == tc.ChannelId {
							channel = single
							break
						}
					}
				}
			}

			return false, err
		})

		return channel, channel != nil, err
	})

	if err != nil {
		return nil, types.ErrInvalidChannel
	}

	return cache.(*telegram.Channel), nil
}

func (tc *TelegramClient) GetTopics(ctx context.Context, search string) ([]*telegram.ForumTopicObj, error) {
	topics, err := tc.topics.Get(search, func(key string) (interface{}, bool, error) {
		var topics []*telegram.ForumTopicObj = nil

		err := tc.pacer.Call(func() (bool, error) {
			mtproto, err := tc.MTProto()
			if err != nil {
				return false, err
			}

			channel, err := tc.GetChannel(ctx)
			if err != nil {
				return false, err
			}

			forum, err := mtproto.ChannelsGetForumTopics(&telegram.ChannelsGetForumTopicsParams{
				Channel: &telegram.InputChannelObj{
					AccessHash: channel.AccessHash,
					ChannelID:  channel.ID,
				},
				Limit: math.MaxInt32,
				Q:     search,
			})

			if cause, ok := errors.Cause(err).(*gogram.ErrResponseCode); ok {
				// ? Check if the error is a flood wait.
				if cause.Code == 420 {
					fs.LogPrint(fs.LogLevelWarning, err.Error())
					return true, cause
				}
			}

			if forum != nil {
				topics = make([]*telegram.ForumTopicObj, len(forum.Topics))
				for i := range forum.Topics {
					if topic, ok := forum.Topics[i].(*telegram.ForumTopicObj); ok {
						topics[i] = topic
					}
				}
			}

			return false, err
		})

		return topics, topics != nil, err
	})

	if err != nil {
		return make([]*telegram.ForumTopicObj, 0), err
	}

	return topics.([]*telegram.ForumTopicObj), err
}

func (tc *TelegramClient) SearchMessagesTopic(ctx context.Context, topic *telegram.ForumTopicObj, search string, offset int32) ([]telegram.Message, int64, bool, int32, error) {
	defer tc.lockFiles.Unlock()
	tc.lockFiles.Lock()

	var response SearchMessagesTopicReturn

	err := tc.pacer.Call(func() (bool, error) {
		mtproto, err := tc.MTProto()
		if err != nil {
			return false, err
		}

		channel, err := tc.GetChannel(ctx)
		if err != nil {
			return false, err
		}

		messages, err := mtproto.MessagesSearch(&telegram.MessagesSearchParams{
			Peer: &telegram.InputPeerChannel{
				AccessHash: channel.AccessHash,
				ChannelID:  channel.ID,
			},
			Filter:   &telegram.InputMessagesFilterEmpty{},
			Limit:    math.MaxInt32,
			TopMsgID: topic.ID,
			OffsetID: offset,
			Q:        search,
		})

		if cause, ok := errors.Cause(err).(*gogram.ErrResponseCode); ok {
			// ? Check if the error is a flood wait.
			if cause.Code == 420 {
				fs.LogPrint(fs.LogLevelWarning, err.Error())
				return true, cause
			}
		}

		switch typed := messages.(type) {
		case *telegram.MessagesMessagesObj:
			response = SearchMessagesTopicReturn{
				Messages:   typed.Messages,
				Amount:     int64(len(typed.Messages)),
				Incomplete: false,
				Offset:     offset,
				Error:      nil,
			}
		case *telegram.MessagesMessagesSlice:
			response = SearchMessagesTopicReturn{
				Messages:   typed.Messages,
				Amount:     int64(len(typed.Messages)),
				Incomplete: true,
				Offset:     typed.OffsetIDOffset,
				Error:      nil,
			}
		case *telegram.MessagesChannelMessages:
			response = SearchMessagesTopicReturn{
				Messages:   typed.Messages,
				Amount:     int64(typed.Count),
				Incomplete: 0 < typed.Count,
				Offset:     typed.OffsetIDOffset,
				Error:      nil,
			}
		case *telegram.MessagesMessagesNotModified:
			response = SearchMessagesTopicReturn{
				Messages:   nil,
				Amount:     int64(typed.Count),
				Incomplete: true,
				Offset:     offset,
				Error:      nil,
			}
		default:
			response = SearchMessagesTopicReturn{
				Messages:   nil,
				Amount:     0,
				Incomplete: false,
				Offset:     offset,
				Error:      types.ErrOperationWithoutUpdates,
			}
		}

		return false, response.Error
	})

	if err != nil {
		return nil, 0, false, offset, err
	}

	return response.Messages,
		response.Amount,
		response.Incomplete,
		response.Offset,
		response.Error
}

// Delete a topic from a channel.
//   - The topic is deleted from the channel.
//   - The request is paced to avoid flooding the server.
//   - If the topic is the default topic (root), it cannot be deleted.
//
// Definition:
//
//	DeleteTopic(ctx context.Context, topic *telegram.ForumTopicObj) error
//
// Returns:
//
//	error - If an error occurs while deleting the topic.
func (tc *TelegramClient) DeleteTopic(ctx context.Context, topic *telegram.ForumTopicObj) error {
	defer tc.lockDirectories.Unlock()
	tc.lockDirectories.Lock()
	var err error

	// ? Prevent the default topic from being deleted.
	if topic.ID == types.ChannelRootTopicId {
		return types.ErrUnsupportedOperation
	}

	err = tc.pacer.Call(func() (bool, error) {
		mtproto, err := tc.MTProto()
		if err != nil {
			return false, err
		}

		channel, err := tc.GetChannel(ctx)
		if err != nil {
			return false, err
		}

		_, err = mtproto.ChannelsDeleteTopicHistory(&telegram.InputChannelObj{
			AccessHash: channel.AccessHash,
			ChannelID:  channel.ID,
		}, topic.ID)

		if cause, ok := errors.Cause(err).(*gogram.ErrResponseCode); ok {
			// ? Check if the error is a flood wait.
			if cause.Code == 420 {
				fs.LogPrint(fs.LogLevelWarning, err.Error())
				return true, cause
			}
		}

		return false, err
	})

	return err
}
