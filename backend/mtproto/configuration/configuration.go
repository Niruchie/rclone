package configuration

import (
	"context"
	"fmt"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/rclone/rclone/backend/mtproto/configuration/logging"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
)

// Fetch the token from the MTProto API.
//
// Definition:
//
//	fetchToken(ctx context.Context, m configmap.Mapper) (*fs.ConfigOut, error)
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
		return &fs.ConfigOut{
			State: "exception",
			Error: err.Error(),
		}, err
	}

	// ? Authorize the client into MTProto API.
	_, err = service.Authorize()
	if err != nil {
		return &fs.ConfigOut{
			State: "exception",
			Error: logging.ErrOTPNotAccepted.Error(),
		}, err
	}

	// ? Get the session token from the MTProto API.
	if mtproto, err := service.Client(); err == nil {
		session := mtproto.
			ExportRawSession().
			Encode()
		m.Set("string_session", session)

		// ? Continue with next step.
		return fs.ConfigResult("dialog_filter_selection", session)
	} else {
		return fs.ConfigError("exception", err.Error())
	}
}

// Select a dialog filter (folder) to use with the bot.
//
// Definition:
//
//	selectDialogFilter(ctx context.Context, m configmap.Mapper) (*fs.ConfigOut, error)
//
// Parameters:
//
//	ctx: context.Context - The context of the request. | Used to cancel the request.
//	m: configmap.Mapper - The configuration map pointer. | Allows to set and get values from the configuration.
//
// This function will fetch the dialog filters from the MTProto API and present them to the user for selection.
func selectDialogFilter(_ context.Context, m configmap.Mapper) (*fs.ConfigOut, error) {
	// ? Parse the config into the struct
	service := &MTProtoService{}
	err := configstruct.Set(m, &(service.Options))
	if err != nil {
		return &fs.ConfigOut{
			State: "exception",
			Error: err.Error(),
		}, err
	}

	// ? Authorize the client into MTProto API.
	_, err = service.Authorize()
	if err != nil {
		return &fs.ConfigOut{
			State: "exception",
			Error: logging.ErrOTPNotAccepted.Error(),
		}, err
	}

	client, err := service.Client()
	defer client.Disconnect()

	if err != nil {
		return &fs.ConfigOut{
			State: "exception",
			Error: err.Error(),
		}, err
	}

	filters, err := client.MessagesGetDialogFilters()
	if err != nil {
		return &fs.ConfigOut{
			State: "exception",
			Error: err.Error(),
		}, err
	}

	var bucketFilterOptions []fs.OptionExample = []fs.OptionExample{}
	for _, filter := range filters.Filters {
		if next, ok := filter.(*telegram.DialogFilterObj); ok {
			bucketFilterOptions = append(bucketFilterOptions, fs.OptionExample{
				Help:     fmt.Sprintf("%s (ID: %d)", next.Title, next.ID),
				Value:    fmt.Sprintf("%d", next.ID),
				Provider: "mtproto",
			})
		}
	}

	return fs.ConfigChooseExclusiveFixed(
		"dialog_filter_set", "dialog_filter_id",
		"Select the dialog filter (folder) to use with the bot",
		bucketFilterOptions,
	)
}

// Select filesystem managers (bots, remotes) to use with the bot.
//
// Definition:
//
//	selectFilesystemManagers(ctx context.Context, m configmap.Mapper, configIn fs.ConfigIn) (*fs.ConfigOut, error)
//
// Parameters:
//
//	ctx: context.Context - The context of the request. | Used to cancel the request.
//	m: configmap.Mapper - The configuration map pointer. | Allows to set and get values from the configuration.
//	configIn: fs.ConfigIn - The configuration input. | Contains the state and result of the configuration.
//
// This function will fetch the filesystem managers from the MTProto API and present them to the user for selection.
func selectFilesystemManagers(ctx context.Context, m configmap.Mapper, configIn fs.ConfigIn) (*fs.ConfigOut, error) {
	var managers fs.SpaceSepList
	err := managers.Set(configIn.Result)
	if err != nil {
		return fs.ConfigError("exception", err.Error())
	}

	bslist, err := cache.GetArr(ctx, managers)
	if err != nil {
		fs.Printf(logging.LoggerString(err), err.Error())
		return fs.ConfigResult("fs_managers_selection", managers.String())
	}

	if len(bslist) < len(managers) || len(bslist) <= 0 {
		fs.Printf(logging.LoggerString(bslist), logging.ErrFilesystemsNotAvailable.Error())
		return fs.ConfigResult("fs_managers_selection", managers.String())
	}

	for _, bs := range bslist {
		name := bs.Name()
		if name != "memory" && name != "mtprotobot" {
			fs.Printf(logging.LoggerString(bs), logging.ErrManagerFilesystemNotSupported.Error())
			return fs.ConfigResult("fs_managers_selection", managers.String())
		}
	}

	m.Set("managers", managers.String())
	return fs.ConfigResult("finished", managers.String())
}

// Configuration function for the MTProto backend.
//
// Definition:
//
//	Configuration(ctx context.Context, name string, m configmap.Mapper, configIn fs.ConfigIn) (*fs.ConfigOut, error)
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
func Configuration(ctx context.Context, name string, m configmap.Mapper, configIn fs.ConfigIn) (*fs.ConfigOut, error) {
	// ? Parse the config into the struct
	params := &MTProtoService{}
	err := configstruct.Set(m, &(params.Options))
	if err != nil {
		return nil, err
	}

	// ? Redirect to the appropriate step based on the state.
	switch configIn.State {
	case "":
		return fetchToken(ctx, m)
	case "dialog_filter_selection":
		return selectDialogFilter(ctx, m)
	case "dialog_filter_set":
		m.Set("dialog_filter_id", configIn.Result)
		return fs.ConfigResult("fs_managers_selection", configIn.Result)
	case "fs_managers_selection":
		return fs.ConfigInputOptional(
			"fs_managers_set", "managers",
			"Select the filesystem managers (bots, remotes) to use",
		)
	case "fs_managers_set":
		return selectFilesystemManagers(ctx, m, configIn)
	case "exception":
	case "finished":
		return nil, nil
	}

	return nil, fmt.Errorf("unexpected state %q", configIn.State)
}
