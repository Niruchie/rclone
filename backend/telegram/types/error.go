package types

import "errors"

// ? filesystem module errors
var ErrInvalidChannel error = errors.New("the channel is invalid or inexistent, check your configuration and bot join status")
var ErrUnsupportedOperation error = errors.New("the operation is not supported by the filesystem")
var ErrOperationWithoutUpdates error = errors.New("the operation was executed without any updates returned")

// ? api module errors
var ErrInvalidBase64PublicKey error = errors.New("the base64 public key is invalid, get and convert it from my.telegram.org")
var ErrInvalidRSAPublicKey error = errors.New("the public key is invalid, cannot find the RSA PEM block")
var ErrInvalidClient error = errors.New("cannot create a new Telegram API client, check your credentials and configuration")
var ErrInvalidClientCouldNotConnect error = errors.New("could not connect to the Telegram MTProtoAPI, check your credentials and configuration")
var ErrInvalidClientCouldNotConnectBot error = errors.New("could not connect to the Telegram Bot API, check your API token on configuration")

// ? configuration errors
var ErrOTPNotAccepted error = errors.New("the two-factor authentication code was not accepted")
var ErrInvalidConfiguration error = errors.New("the configuration is invalid, check your configuration")
var ErrInvalidNoChannelsFound error = errors.New("no channels were found, join the bot to a channel on Telegram and try again")
