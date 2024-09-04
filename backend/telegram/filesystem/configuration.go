package filesystem

import (
	"context"
	"fmt"
	"math"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/rclone/rclone/backend/telegram/api"
	"github.com/rclone/rclone/backend/telegram/types"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
)

// Fetch the token from the Telegram MTProto API.
//
// Definition:
//    fetchTokenMTProto(m *configmap.Mapper) (*fs.ConfigOut, error)
//
// Parameters:
//    m: *configmap.Mapper - The configuration map pointer. | Allows to set and get values from the configuration.
//
// This function will fetch the token from the Telegram MTProto API.
// It will ask for the phone number and the two-factor authentication code if needed.
// Then it will store the session token in the configuration map, to be used in the next steps.
func fetchTokenMTProto(m *configmap.Mapper) (*fs.ConfigOut, error) {
	// ? Parse the config into the struct
	client := &api.TelegramClient{}
	err := configstruct.Set(*m, &(client.Options))
	if err != nil {
		return &fs.ConfigOut{
			State: "exception",
			Error: err.Error(),
		}, err
	}

	// ? Get client from the api module.
	_, err = client.Authorize()
	if err != nil {
		return &fs.ConfigOut{
			State: "exception",
			Error: types.ErrOTPNotAccepted.Error(),
		}, err
	}

	// ? Get the session token from the MTProto API.
	session := client.
		MTProto().
		ExportRawSession().
		Encode()
	(*m).Set("string_session", session)

	// ? Continue with next step.
	return &fs.ConfigOut{
		State:  "channel_select",
		Result: session,
	}, nil
}

// Select the channel to use with the bot.
//
// Definition:
//    selectChannelWithBot(m *configmap.Mapper) (*fs.ConfigOut, error)
//
// Parameters:
//    m: *configmap.Mapper - The configuration map pointer. | Allows to set and get values from the configuration.
//
// This function will fetch the channels from the Telegram MTProto API
// and check if the bot is within any of them, then offer the user to select
// one of the channels to use with rclone client.
func selectChannelWithBot(m *configmap.Mapper) (*fs.ConfigOut, error) {
	// ? Parse the config into the struct
	client := &api.TelegramClient{}
	err := configstruct.Set(*m, &(client.Options))
	if err != nil {
		return &fs.ConfigOut{
			State: "exception",
			Error: err.Error(),
		}, err
	}

	// ? Connect Telegram MTProto API and Bot API.
	client.Connect()
	bot := client.Bot()
	mtproto := client.MTProto()

	// ? Get the MeID from the bot.
	me, err := bot.GetMe()

	if err != nil {
		return &fs.ConfigOut{
			State: "exception",
			Error: err.Error(),
		}, err
	}

	// ? Get the dialogs from the Telegram MTProto API.
	dialogs, err := mtproto.
		GetDialogs(&telegram.DialogOptions{
			Limit: math.MaxInt32,
		})

	if err != nil {
		return &fs.ConfigOut{
			State: "exception",
			Error: err.Error(),
		}, err
	}

	// ? Store channels to fetch if bot is admin in any of them.
	var foundChannels []int64

	for _, single := range dialogs {
		if dialog, ok := single.(*telegram.DialogObj); ok {
			if dialog.Peer == nil {
				log := fmt.Sprintf("The following dialog has a nil peer: %v", dialog)
				fs.LogPrint(fs.LogLevelError, log)
				continue
			}

			if next, ok := dialog.Peer.(*telegram.PeerChannel); ok {
				peer, err := mtproto.ResolvePeer(next.ChannelID)
				if err != nil {
					log := fmt.Sprintf("Error resolving peer: %v", err)
					fs.LogPrint(fs.LogLevelError, log)
					continue 
				}

				if channelPeer, ok := peer.(*telegram.InputPeerChannel); ok {
					full, err := mtproto.
						ChannelsGetFullChannel(
							&telegram.InputChannelObj{
								ChannelID:  channelPeer.ChannelID,
								AccessHash: channelPeer.AccessHash,
							},
						)

					if err != nil {
						log := fmt.Sprintf("Error getting full channel: %v", err)
						fs.LogPrint(fs.LogLevelError, log)
						continue
					}

					for _, user := range full.Users {
						if user.(*telegram.UserObj).ID == me.ID {
							foundChannels = append(foundChannels, channelPeer.ChannelID)
						}
					}
				}
			}
		}
	}

	if len(foundChannels) <= 0 {
		return &fs.ConfigOut{
			State: "exception",
			Error: types.ErrInvalidNoChannelsFound.Error(),
		}, types.ErrInvalidNoChannelsFound
	}

	var channelOptions []fs.OptionExample = []fs.OptionExample{}
	
	// ? Fetch the channel title from the channel ID.
	for _, item := range foundChannels {
		channel, err := mtproto.GetChannel(item)
		if err == nil {
			channelOptions = append(channelOptions, fs.OptionExample{
				Value: fmt.Sprintf("%d", item),
				Help:  channel.Title,
				Provider: "telegram",
			})
		}
	}

	// ? Disconnect the clients on exit.
	defer client.Disconnect()

	return fs.ConfigChooseExclusiveFixed(
		"channel_id_set", "channel_id",
		"Select the channel/chat to use with the bot",
		channelOptions,
	)
}

// Configuration function for the Telegram backend.
//
// Definition:
//    Configuration(ctx context.Context, name string, m configmap.Mapper, configIn fs.ConfigIn) (*fs.ConfigOut, error)
//
// Parameters:
//    ctx: context.Context - The context of the configuration. | Used to cancel the configuration.
//    name: string - The name of the backend. | Used to identify the backend.
//    m: configmap.Mapper - The configuration map. | Allows to set and get values from the configuration.
//    configIn: fs.ConfigIn - The configuration input. | Contains the state and result of the configuration.
//
// This function will handle the configuration of the Telegram backend.
// It will redirect to the appropriate step based on the state.
// Also receive the result of each step to pass into the next one.
// Finally, it will return the configuration output to the rclone client.
func Configuration(ctx context.Context, name string, m configmap.Mapper, configIn fs.ConfigIn) (*fs.ConfigOut, error) {
	// ? Parse the config into the struct
	params := &api.TelegramClient{}
	err := configstruct.Set(m, &(params.Options))
	if err != nil {
		return nil, err
	}

	// ? Redirect to the appropriate step based on the state.
	switch configIn.State {
	case "":
		return fetchTokenMTProto(&m)
	case "channel_select":
		return selectChannelWithBot(&m)
	case "channel_id_set":
		m.Set("channel_id", configIn.Result)
		return &fs.ConfigOut{
			State:  "finished",
			Result: configIn.Result,
		}, nil
	case "exception":
	case "finished":
		return nil, nil
	}

	return nil, fmt.Errorf("unexpected state %q", configIn.State)
}