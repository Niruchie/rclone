package types

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/amarnathcjd/gogram"
)

// LoggerString returns a string for logging value structures.
//
// Definition:
//  func LoggerString(o interface{}) string
//
// Parameters:
//  o: interface{} - the value to be converted to a string
//
// Returns:
//  string - the string representation of the value
//
// The function uses reflection whether the value is an expected type or not.
func LoggerString(o interface{}) string {
	switch target := o.(type) {
	case error:
		var cause *gogram.ErrResponseCode
		if errors.As(target, &cause) {
			return fmt.Sprintf("telegram (%s: %d)", cause.Message, cause.Code)
		}

		return fmt.Sprintf("telegram (error: %s)", target)
	default:
		val := reflect.ValueOf(o)
		switch val.Kind() {
		case reflect.String:
			return fmt.Sprintf("telegram (string: %s)", val.String())
		case reflect.Float32, reflect.Float64:
			return fmt.Sprintf("telegram (float: %f)", val.Float())
		case reflect.Bool:
			return fmt.Sprintf("telegram (bool: %t)", val.Bool())
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return fmt.Sprintf("telegram (int: %d)", val.Int())
		default:
			return fmt.Sprintf("telegram <type: %T>", target)
		}
	}
}
// filesystem module errors
var (	
	ErrInvalidChannel          = errors.New("the channel is invalid or inexistent, check your configuration and bot join status")
	ErrUnsupportedOperation    = errors.New("the operation is not supported by the filesystem")
	ErrOperationWithoutUpdates = errors.New("the operation was executed without any updates returned")
)

// api module errors
var (
	ErrInvalidBase64PublicKey          = errors.New("the base64 public key is invalid, get and convert it from my.telegram.org")
	ErrInvalidRSAPublicKey             = errors.New("the public key is invalid, cannot find the RSA PEM block")
	ErrInvalidClient                   = errors.New("cannot create a new Telegram API client, check your credentials and configuration")
	ErrInvalidClientCouldNotConnect    = errors.New("could not connect to the Telegram MTProtoAPI, check your credentials and configuration")
	ErrInvalidClientCouldNotConnectBot = errors.New("could not connect to the Telegram Bot API, check your API token on configuration")
)

// configuration errors
var (
	ErrOTPNotAccepted         = errors.New("the two-factor authentication code was not accepted")
	ErrInvalidConfiguration   = errors.New("the configuration is invalid, check your configuration")
	ErrInvalidNoChannelsFound = errors.New("no channels were found, join the bot to a channel on Telegram and try again")
)
