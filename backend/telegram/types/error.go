package types

import "errors"

// ? filesystem module errors
var ErrInvalidDatabase error = errors.New("the database connection string does not match any supported database type")
var ErrInvalidChannel error = errors.New("the channel is invalid or inexistent, check your configuration and bot join status")
var ErrChannelNotFound error = errors.New("the channel was not found")
var ErrOTPNotSent error = errors.New("the two-factor authentication code was not sent")
var ErrOTPNotAccepted error = errors.New("the two-factor authentication code was not accepted")

// ? api module errors
var ErrInvalidBase64PublicKey error = errors.New("the base64 public key is invalid, get and convert it from my.telegram.org")
var ErrInvalidRSAPublicKey error = errors.New("the public key is invalid, cannot find the RSA PEM block")
var ErrInvalidClient error = errors.New("cannot create a new Telegram API client, check your credentials and configuration")
var ErrInvalidClientCouldNotConnect error = errors.New("could not connect to the Telegram MTProtoAPI, check your credentials and configuration")
var ErrInvalidClientCouldNotConnectBot error = errors.New("could not connect to the Telegram Bot API, check your API token on configuration")

// ? configuration errors
var ErrInvalidConfiguration error = errors.New("the configuration is invalid, check your configuration")
var ErrInvalidNoChannelsFound error = errors.New("no channels were found, join the bot to a channel on Telegram and try again")
