package api

import (
	"math"
	"math/rand"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/rclone/rclone/backend/telegram/types"
)

func GetChannel(mtproto *telegram.Client, channelId int64) (*telegram.Channel, error) {
	var channel *telegram.Channel

	typeMessagesChats, err := mtproto.
		ChannelsGetChannels([]telegram.InputChannel{
			&telegram.InputChannelObj{
				ChannelID: channelId,
			},
		})

	if err != nil {
		return nil, err
	}

	if messagesChats, ok := typeMessagesChats.(*telegram.MessagesChatsObj); ok {
		for _, chat := range messagesChats.Chats {
			if chatChannel, ok := chat.(*telegram.Channel); ok {
				if chatChannel.ID == channelId {
					channel = chatChannel
					break
				}
			}
		}
	}

	if channel == nil {
		return nil, types.ErrInvalidChannel
	}

	return channel, nil
}

func GetTopics(mtproto *telegram.Client, channel *telegram.Channel, search string) ([]*telegram.ForumTopicObj, error) {
	forum, err := mtproto.ChannelsGetForumTopics(&telegram.ChannelsGetForumTopicsParams{
		Channel: &telegram.InputChannelObj{
			AccessHash: channel.AccessHash,
			ChannelID:  channel.ID,
		},
		Limit: math.MaxInt32,
		Q:     search,
	})

	if err != nil {
		return nil, err
	}

	topics := make([]*telegram.ForumTopicObj, len(forum.Topics))
	for i := range forum.Topics {
		topics[i] = forum.Topics[i].(*telegram.ForumTopicObj)
	}

	return topics, err
}

func CreateTopic(mtproto *telegram.Client, channel *telegram.Channel, title string) (*telegram.ForumTopicObj, bool, error) {
	// ? Check whether the topic already exists.
	topics, err := GetTopics(mtproto, channel, title)
	if err != nil {
		return nil, false, err
	}

	for _, topic := range topics {
		if topic.Title == title {
			return topic, false, nil
		}
	}

	// ? Create the topic if it does not exist.
	updates, err := mtproto.ChannelsCreateForumTopic(&telegram.ChannelsCreateForumTopicParams{
		Channel: &telegram.InputChannelObj{
			AccessHash: channel.AccessHash,
			ChannelID:  channel.ID,
		},
		RandomID: rand.Int63(),
		Title:    title,
	})

	if err != nil {
		return nil, false, err
	}

	// ? Read all the updates and message from service, creating a new topic.
	// * Telegram creates a service message on channel,
	// * converting that service message creates a topic which can be replied.
	if updates, ok := updates.(*telegram.UpdatesObj); ok {
		for _, update := range updates.Updates {
			if message, ok := update.(*telegram.UpdateNewChannelMessage); ok {
				if serviceMsg, ok := message.Message.(*telegram.MessageService); ok {
					return &telegram.ForumTopicObj{
						FromID:         serviceMsg.PeerID,
						Date:           serviceMsg.Date,
						ID:             serviceMsg.ID,
						ReadInboxMaxID: serviceMsg.ID,
						TopMessage:     serviceMsg.ID,
						Title:          title,
						My:             true,
					}, true, nil
				}
			}
		}
	}

	return nil, false, types.ErrOperationWithoutUpdates
}

// ? Delete a topic from a super group chat | channel.
// * If the topic is the default topic (1), it cannot be deleted, unsupported operation.
func DeleteTopic(mtproto *telegram.Client, channel *telegram.Channel, topic *telegram.ForumTopicObj) error {
	if topic.ID == 1 {
		return types.ErrUnsupportedOperation
	}

	affected, err := mtproto.ChannelsDeleteTopicHistory(&telegram.InputChannelObj{
		AccessHash: channel.AccessHash,
		ChannelID:  channel.ID,
	}, topic.ID)

	if err != nil {
		return err
	}

	_ = affected
	return nil
}

func SearchMessagesTopic(mtproto *telegram.Client, channel *telegram.Channel, topic *telegram.ForumTopicObj, search string, offset int32) ([]telegram.Message, int64, bool, int32, error) {
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

	if err != nil {
		return nil, 0, false, offset, err
	}

	switch typed := messages.(type) {

	// ? Whether the messages come completely, third parameter is false.
	case *telegram.MessagesMessagesObj:
		return typed.Messages, int64(len(typed.Messages)), false, offset, nil

	// ? Whether the messages come incompletely, third parameter is true.
	case *telegram.MessagesMessagesSlice:

		return typed.Messages, int64(len(typed.Messages)), true, typed.OffsetIDOffset, nil

	// ? Whether the messages come from a channel or a super group chat.
	case *telegram.MessagesChannelMessages:
		return typed.Messages, int64(typed.Count), 0 < typed.Count, typed.OffsetIDOffset, nil

	// ? This one works when the query is repeated from time to time.
	case *telegram.MessagesMessagesNotModified:
		return nil, int64(typed.Count), true, offset, nil
	default:
		return nil, 0, false, offset, types.ErrOperationWithoutUpdates

	}
}
