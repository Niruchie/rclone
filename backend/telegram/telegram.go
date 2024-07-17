package telegram

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

var options = []fs.Option{
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

func fetchTokenMTProto(m *configmap.Mapper) (*fs.ConfigOut, error) {
	// ? Parse the config into the struct
	options := &types.Options{}
	err := configstruct.Set(*m, options)
	if err != nil {
		return &fs.ConfigOut{
			State: "exception",
			Error: err.Error(),
		}, err
	}

	// ? Get client from the api module.
	client, err := api.GetClientMTProto(
		options.AppId,
		options.AppHash,
		options.PublicKey,
		func() string {
			// ? Whether to use the old string session or not.
			if options.StringSession == types.SessionStringEmpty {
				return types.SessionStringEmpty
			} else {
				return options.StringSession
			}
		}(),
	)

	if err != nil {
		return nil, err
	}

	// ? Sign in with the code.
	_, err = client.Login(options.PhoneNumber, &telegram.LoginOptions{})

	if err != nil {
		return &fs.ConfigOut{
			State: "exception",
			Error: types.ErrOTPNotAccepted.Error(),
		}, err
	}

	// ? Get the session token from the MTProto API.
	session := client.ExportRawSession().Encode()
	(*m).Set("string_session", session)

	// ? Continue with next step.
	return &fs.ConfigOut{
		State:  "channel_select",
		Result: session,
	}, nil
}

func selectChannelWithBot(m *configmap.Mapper) (*fs.ConfigOut, error) {
	// ? Parse the config into the struct
	options := &types.Options{}
	err := configstruct.Set(*m, options)
	if err != nil {
		return &fs.ConfigOut{
			State: "exception",
			Error: err.Error(),
		}, err
	}

	// ? Get client for the Telegram MTProto API.
	// ? From the previous step [StringSession].
	mtproto, err := api.GetClientMTProto(
		options.AppId,
		options.AppHash,
		options.PublicKey,
		options.StringSession,
	)

	if err != nil {
		return &fs.ConfigOut{
			State: "exception",
			Error: err.Error(),
		}, err
	}

	// ? Get client for the Telegram Bot API.
	bot, err := api.GetClientBot(
		options.AppId,
		options.AppHash,
		options.PublicKey,
		options.BotToken,
	)

	if err != nil {
		return &fs.ConfigOut{
			State: "exception",
			Error: err.Error(),
		}, err
	}

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

	for _, dialog := range dialogs {
		switch dialog := dialog.(type) {
		// ? Check whether the result casts to a DialogObj.
		case *telegram.DialogObj:
			switch next := dialog.Peer.(type) {

			// ? Only channels are supported, cast to PeerChannel.
			case *telegram.PeerChannel:
				peer, err := mtproto.ResolvePeer(next.ChannelID)
				if err != nil {
					fs.Debugf("telegram", "Error resolving peer: %v", err)
				}

				// ? Cast the resolved peer to InputPeerChannel.
				if channelPeer, ok := peer.(*telegram.InputPeerChannel); ok {
					full, err := mtproto.
						ChannelsGetFullChannel(
							&telegram.InputChannelObj{
								ChannelID:  channelPeer.ChannelID,
								AccessHash: channelPeer.AccessHash,
							},
						)

					if err != nil {
						fs.Debugf("telegram", "Error getting full channel: %v", err)
					}

					// ? Check if the bot exists within the channel.
					for _, user := range full.Users {
						if user.(*telegram.UserObj).ID == me.ID {
							foundChannels = append(foundChannels, channelPeer.ChannelID)
						}
					}
				}
			default:
				fs.Debugf("telegram", "The following peer is not a PeerChannel: %v", dialog)
			}
		default:
			fs.Debugf("telegram", "The following dialog is not a DialogObj: %v", dialog)
		}
	}

	if len(foundChannels) <= 0 {
		return &fs.ConfigOut{
			State: "exception",
			Error: types.ErrInvalidNoChannelsFound.Error(),
		}, types.ErrInvalidNoChannelsFound
	}

	var list []fs.OptionExample = []fs.OptionExample{}

	// ? Fetch the channel title from the channel ID.
	for _, item := range foundChannels {
		channel, err := mtproto.GetChannel(item)
		if err == nil {
			list = append(list, fs.OptionExample{
				Value: fmt.Sprintf("%d", item),
				Help:  channel.Title,
			})
		}
	}

	// ? Disconnect the clients on exit.
	defer mtproto.Disconnect()
	defer bot.Disconnect()

	return fs.ConfigChooseExclusiveFixed(
		"channel_id_set", "channel_id",
		"Select the channel/chat to use with the bot",
		list,
	)
}

func configuration(ctx context.Context, name string, m configmap.Mapper, configIn fs.ConfigIn) (*fs.ConfigOut, error) {
	// ? Parse the config into the struct
	params := &types.Options{}
	err := configstruct.Set(m, params)
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

func init() {
	fs.Register(
		&fs.RegInfo{
			Config:      configuration,
			Description: "Telegram",
			Name:        "telegram",
			Options:     options,
		},
	)
}
