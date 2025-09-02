package configuration

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/rclone/rclone/backend/mtproto/configuration/logging"
	"github.com/rclone/rclone/backend/mtproto/configuration/options"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
)

// Fetch the token from the MTProto API.
//
// Parameters:
//
//	ctx: context.Context - The context of the request. | Used to cancel the request.
//	m: configmap.Mapper - The configuration map pointer. | Allows to set and get values from the configuration.
//
// This function will fetch the token from the MTProto API.
// It will ask for the phone number and the two-factor authentication code if needed.
// Then it will store the session token in the configuration map, to be used in the next steps.
func fetchToken(_ context.Context, m configmap.Mapper) (*fs.ConfigOut, error) {
	// ? Parse the config into the struct
	service := &MTProtoService{}
	err := configstruct.Set(m, &(service.Options))
	if err != nil {
		return fs.ConfigError(StepException.String(), err.Error())
	}

	// ? Authorize the client into MTProto API.
	_, err = service.Authorize()
	if err != nil {
		return fs.ConfigError(StepException.String(), err.Error())
	}

	// ? Get the session token from the MTProto API.
	if mtproto, err := service.Client(); err == nil {
		session := mtproto.ExportRawSession().Encode()
		m.Set(InputStringSession.String(), session)

		// ? Continue with next step.
		return fs.ConfigGoto(StepShouldCreateSupergroupForum.String())
	} else {
		return fs.ConfigError(StepException.String(), err.Error())
	}
}

// Create a new Supergroup Forum using the MTProto API.
//
// Parameters:
//
//	ctx: context.Context - The context of the request. | Used to cancel the request.
//	m: configmap.Mapper - The configuration map pointer. | Allows to set and get values from the configuration.
//	name: string - The name of the supergroup forum to create. | Used to identify the forum.
//
// This function will create a new supergroup forum using the MTProto API.
func createSupergroupForum(ctx context.Context, m configmap.Mapper, configIn fs.ConfigIn) (*fs.ConfigOut, error) {
	// ? Parse the config into the struct
	service := &MTProtoService{}
	err := configstruct.Set(m, &(service.Options))
	if err != nil {
		return fs.ConfigError(StepException.String(), err.Error())
	}

	// ? Authorize the client into MTProto API.
	_, err = service.Authorize()
	if err != nil {
		return fs.ConfigError(StepException.String(), err.Error())
	}

	// ? Create the supergroup supergroup.
	name := strings.TrimSpace(configIn.Result)
	supergroup, created, err := service.CreateChannel(ctx, name)
	if err != nil {
		return fs.ConfigError(StepException.String(), err.Error())
	}

	if !created {
		return fs.ConfigError(StepException.String(), logging.ErrInvalidChannel.Error())
	}

	m.Set(InputSupergroupId.String(), fmt.Sprintf("%d", supergroup.ID))
	return fs.ConfigResult(StepManagerInput.String(), fmt.Sprintf("%d", supergroup.ID))
}

// Create a new Supergroup Forum using the MTProto API.
//
// Parameters:
//
//	_: context.Context - The context of the request. | Used to cancel the request.
//	m: configmap.Mapper - The configuration map pointer. | Allows to set and get values from the configuration.
//
// This function will create a new supergroup forum using the MTProto API.
func chooseSupergroupForum(_ context.Context, m configmap.Mapper) (*fs.ConfigOut, error) {
	service := &MTProtoService{}
	err := configstruct.Set(m, &(service.Options))
	if err != nil {
		return fs.ConfigError(StepException.String(), err.Error())
	}

	// ? Authorize the client into MTProto API.
	_, err = service.Authorize()
	if err != nil {
		return fs.ConfigError(StepException.String(), err.Error())
	}

	client, err := service.Client()
	defer client.Disconnect()
	if err != nil {
		return fs.ConfigError(StepException.String(), err.Error())
	}

	dialogs, err := client.GetDialogs(&telegram.DialogOptions{
		Limit: math.MaxInt32,
	})
	if err != nil {
		return fs.ConfigError(StepException.String(), err.Error())
	}

	var channelSearchInputs []telegram.InputChannel = []telegram.InputChannel{}
	for _, next := range dialogs {
		var dialog *telegram.DialogObj
		switch d := next.(type) {
		case *telegram.DialogObj:
			dialog = d
		default:
			continue
		}

		switch dialogPeer := dialog.Peer.(type) {
		case *telegram.PeerChannel:
			peer, err := client.ResolvePeer(dialogPeer.ChannelID)
			if err != nil {
				fs.Error(logging.LoggerString(dialogPeer), err.Error())
				continue
			}

			if inputPeerChannel, ok := peer.(*telegram.InputPeerChannel); ok {
				channelSearchInputs = append(channelSearchInputs, &telegram.InputChannelObj{
					AccessHash: inputPeerChannel.AccessHash,
					ChannelID:  inputPeerChannel.ChannelID,
				})
			}
		default:
			continue
		}
	}

	if len(channelSearchInputs) <= 0 {
		return fs.ConfigError(StepException.String(), logging.ErrInvalidNoChannelsFound.Error())
	}

	var options []fs.OptionExample = []fs.OptionExample{}
	response, err := client.ChannelsGetChannels(channelSearchInputs)
	if err != nil {
		return fs.ConfigError(StepException.String(), err.Error())
	}

	switch chats := response.(type) {
	case *telegram.MessagesChatsObj:
		for _, chat := range chats.Chats {
			switch channel := chat.(type) {
			case *telegram.Channel:
				if channel.Left || !channel.Forum || !channel.Megagroup {
					continue
				}

				options = append(options, fs.OptionExample{
					Value:    fmt.Sprintf("%d", channel.ID),
					Help:     channel.Title,
					Provider: "mtproto",
				})
			}
		}
	}

	if len(options) <= 0 {
		return fs.ConfigError(StepException.String(), logging.ErrInvalidNoChannelsFound.Error())
	}

	return fs.ConfigChooseExclusiveFixed(
		StepSetSupergroupForum.String(),
		InputSupergroupId.String(),
		"Select the supergroup forum which will act as storage",
		options,
	)
}

// Configures the manager bot tokens to be used on MTProto API on backend.
//
// Parameters:
//
//	_: context.Context - The context of the request. | Used to cancel the request.
//	m: configmap.Mapper - The configuration map pointer. | Allows to set and get values from the configuration.
func configureManagerEditor(_ context.Context, m configmap.Mapper) (*fs.ConfigOut, error) {
	options := &options.Options{}
	err := configstruct.Set(m, options)
	if err != nil {
		return fs.ConfigError(StepException.String(), err.Error())
	}

	o := logging.LoggerString(options.Managers)
	fs.Print(o, "Current manager bot tokens:")
	if len(options.Managers) <= 0 {
		fs.Print(o, "\t No manager bot tokens found.")
	} else {
		for i, manager := range options.Managers {
			fs.Printf(o, "\t %d. %v", i+1, manager)
		}
	}

	choices := []fs.OptionExample{
		{
			Help:     "Input a new manager bot token",
			Value:    StepManagerInput.String(),
			Provider: "mtproto",
		},
		{
			Help:     "Delete an existing manager token",
			Value:    StepManagerRemove.String(),
			Provider: "mtproto",
		},
		{
			Help:     "Exit this manager bot token configuration",
			Value:    StepManagerReady.String(),
			Provider: "mtproto",
		},
	}

	return fs.ConfigChooseExclusiveFixed(
		StepManagerSelect.String(),
		InputAction.String(),
		"Select an action",
		choices,
	)
}

// Tests the provided manager bot token with the MTProto API.
//
// Parameters:
//
//	_: context.Context - The context of the request. | Used to cancel the request.
//	m: configmap.Mapper - The configuration map pointer. | Allows to set and get values from the configuration.
//	configIn: fs.ConfigIn - The configuration input. | "configIn.Result" should contain the manager bot token.
func testActionManagerEditor(_ context.Context, m configmap.Mapper, configIn fs.ConfigIn) (*fs.ConfigOut, error) {
	options := &options.Options{}
	token := strings.TrimSpace(configIn.Result)
	err := configstruct.Set(m, options)
	if err != nil {
		return fs.ConfigError(StepException.String(), err.Error())
	}

	// If token is already in options.Managers, return to Editor
	if slices.Contains(options.Managers, token) {
		return fs.ConfigError(
			StepManagerFail.String(),
			"manager bot token already exists",
		)
	}

	// TODO: better test for the manager bot token
	{
		config := telegram.
			NewClientConfigBuilder(
				options.AppId,
				options.AppHash,
			).
			WithMemorySession().
			WithDisableCache().
			Build()

		client, err := telegram.NewClient(config)
		if err != nil {
			return fs.ConfigError(
				StepManagerFail.String(),
				err.Error(),
			)
		}

		err = client.LoginBot(token)
		defer client.Disconnect()
		if err != nil {
			return fs.ConfigError(
				StepManagerFail.String(),
				err.Error(),
			)
		}

		options.Managers = append(options.Managers, token)
	}

	return fs.ConfigResult(
		StepManagerSet.String(),
		options.Managers.String(),
	)
}

// Requires the user to provide the value of the manager bot token to delete.
//
// Parameters:
//
//	_: context.Context - The context of the request. | Used to cancel the request.
//	m: configmap.Mapper - The configuration map pointer. | Allows to set and get values from the configuration.
func removeActionManagerEditor(_ context.Context, m configmap.Mapper) (*fs.ConfigOut, error) {
	var options = &options.Options{}
	var choices = []fs.OptionExample{}
	err := configstruct.Set(m, options)
	if err != nil {
		return fs.ConfigError(StepException.String(), err.Error())
	}

	for i, manager := range options.Managers {
		choices = append(choices, fs.OptionExample{
			Help:     fmt.Sprintf("Manager bot token %d", i+1),
			Provider: "mtproto",
			Value:    manager,
		})
	}

	if len(choices) <= 0 {
		o := logging.LoggerString(options.Managers)
		fs.Print(o, "No manager bot tokens found to delete.")
		return fs.ConfigGoto(StepManagerEditor.String())
	}

	return fs.ConfigChooseExclusiveFixed(
		StepManagerDelete.String(),
		InputManagerToken.String(),
		"Select a manager bot token to delete",
		choices,
	)
}

// Deletes the provided manager bot token from the configuration.
//
// Parameters:
//
//	_: context.Context - The context of the request. | Used to cancel the request.
//	m: configmap.Mapper - The configuration map pointer. | Allows to set and get values from the configuration.
func deleteActionManagerEditor(_ context.Context, m configmap.Mapper, configIn fs.ConfigIn) (configOut *fs.ConfigOut, err error) {
	var token = configIn.Result
	var options = &options.Options{}
	err = configstruct.Set(m, options)
	if err != nil {
		return fs.ConfigError(StepException.String(), err.Error())
	}

	for i, manager := range options.Managers {
		if manager == token {
			options.Managers = append(
				options.Managers[:i],
				options.Managers[i+1:]...,
			)
			break
		}
	}

	return fs.ConfigResult(
		StepManagerSet.String(),
		options.Managers.String(),
	)
}

// Represents a step in the configuration process.
type ConfigurationStep string

func (step ConfigurationStep) String() string {
	return string(step)
}

// Next configuration step that will be executed.
const (
	StepShouldCreateSupergroupForum = ConfigurationStep("should_create_supergroup_forum")
	StepSelectSupergroupForum       = ConfigurationStep("select_supergroup_forum")
	StepCreateSupergroupForum       = ConfigurationStep("create_supergroup_forum")
	StepChooseSupergroupForum       = ConfigurationStep("choose_supergroup_forum")
	StepSetSupergroupForum          = ConfigurationStep("set_supergroup_forum")
	StepManagerEditor               = ConfigurationStep("manager_editor")
	StepManagerSelect               = ConfigurationStep("manager_select")
	StepManagerRemove               = ConfigurationStep("manager_remove")
	StepManagerDelete               = ConfigurationStep("manager_delete")
	StepManagerReady                = ConfigurationStep("manager_ready")
	StepManagerInput                = ConfigurationStep("manager_input")
	StepManagerTest                 = ConfigurationStep("manager_test")
	StepManagerFail                 = ConfigurationStep("manager_fail")
	StepManagerSet                  = ConfigurationStep("manager_set")
	StepException                   = ConfigurationStep("exception")
	StepFinished                    = ConfigurationStep("finished")
	StepEmpty                       = ConfigurationStep("")
)

// Represents an input field in the configuration process.
type ConfigurationInput string

func (name ConfigurationInput) String() string {
	return string(name)
}

// Next input field that will be set.
const (
	InputSupergroupExclusive = ConfigurationInput("supergroup_exclusive")
	InputSupergroupForum     = ConfigurationInput("supergroup_forum")
	InputSupergroupId        = ConfigurationInput("supergroup_id")
	InputStringSession       = ConfigurationInput("string_session")
	InputManagerToken        = ConfigurationInput("manager_token")
	InputManagers            = ConfigurationInput("managers")
	InputAction              = ConfigurationInput("action")
	ShouldBeFalse            = ConfigurationInput("false")
	ShouldBeTrue             = ConfigurationInput("true")
)

// Configuration function for the MTProto backend.
//
// Parameters:
//
//	ctx: context.Context - The context of the configuration. | Used to cancel the configuration.
//	name: string - The name of the backend. | Used to identify the backend.
//	m: configmap.Mapper - The configuration map. | Allows to set and get values from the configuration.
//	configIn: fs.ConfigIn - The configuration input. | Contains the state and result of the configuration.
//
// This function will handle the configuration of the MTProto backend.
// It will redirect to the appropriate step based on the state.
// Also receive the result of each step to pass into the next one.
// Finally, it will return the configuration output to the rclone client.
func Configuration(ctx context.Context, name string, m configmap.Mapper, configIn fs.ConfigIn) (configOut *fs.ConfigOut, err error) {
	// ? Redirect to the appropriate step based on the state.
	switch ConfigurationStep(configIn.State) {
	case StepEmpty:
		return fetchToken(ctx, m)
	case StepShouldCreateSupergroupForum:
		return fs.ConfigConfirm(
			StepSelectSupergroupForum.String(),
			false, InputSupergroupExclusive.String(),
			"Do you want to create a exclusive supergroup forum to work with?",
		)
	case StepSelectSupergroupForum:
		switch configIn.Result {
		case ShouldBeTrue.String():
			return fs.ConfigInput(
				StepCreateSupergroupForum.String(),
				InputSupergroupForum.String(),
				"Enter the name of the supergroup forum to be created",
			)
		case ShouldBeFalse.String():
			return fs.ConfigGoto(StepChooseSupergroupForum.String())
		}
	case StepCreateSupergroupForum:
		return createSupergroupForum(ctx, m, configIn)
	case StepChooseSupergroupForum:
		return chooseSupergroupForum(ctx, m)
	case StepSetSupergroupForum:
		m.Set(InputSupergroupId.String(), strings.TrimSpace(configIn.Result))
		return fs.ConfigGoto(StepManagerEditor.String())
	case StepManagerEditor:
		return configureManagerEditor(ctx, m)
	case StepManagerSelect:
		switch action := ConfigurationStep(configIn.Result); action {
		case StepManagerInput:
			return fs.ConfigInput(
				StepManagerTest.String(),
				InputManagerToken.String(),
				"Enter the manager bot token to add it",
			)
		case StepManagerRemove:
			return fs.ConfigGoto(StepManagerRemove.String())
		case StepManagerReady:
			return fs.ConfigGoto(StepFinished.String())
		}

		return fs.ConfigError(StepException.String(), "selected an invalid action")
	case StepManagerTest:
		return testActionManagerEditor(ctx, m, configIn)
	case StepManagerFail:
		o := logging.LoggerString(configIn)
		fs.Errorf(o, "failed to add manager bot token: %q:%q", configIn.State, configIn.Result)
		return fs.ConfigGoto(StepManagerEditor.String())
	case StepManagerSet:
		m.Set(InputManagers.String(), strings.TrimSpace(configIn.Result))
		return fs.ConfigGoto(StepManagerEditor.String())
	case StepManagerRemove:
		return removeActionManagerEditor(ctx, m)
	case StepManagerDelete:
		return deleteActionManagerEditor(ctx, m, configIn)
	case StepException:
		return nil, fmt.Errorf("%s", configIn.Result)
	case StepFinished:
		return nil, nil
	}

	log := fmt.Sprintf("unexpected state %q:%q", configIn.State, configIn.Result)
	return fs.ConfigError(StepException.String(), log)
}
