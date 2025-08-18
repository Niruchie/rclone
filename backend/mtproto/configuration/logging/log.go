package logging

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/amarnathcjd/gogram"
)

// LoggerString returns a string for logging value structures.
//
// Definition:
//
//	func LoggerString(o interface{}) string
//
// Parameters:
//
//	o: interface{} - the value to be converted to a string
//
// Returns:
//
//	string - the string representation of the value
//
// The function uses reflection whether the value is an expected type or not.
func LoggerString(o any) string {
	switch target := o.(type) {
	case error:
		var cause *gogram.ErrResponseCode
		if errors.As(target, &cause) {
			return fmt.Sprintf("mtproto (%s: %d)", cause.Message, cause.Code)
		}

		return fmt.Sprintf("mtproto (error: %s)", target)
	default:
		val := reflect.ValueOf(o)
		switch val.Kind() {
		case reflect.String:
			return fmt.Sprintf("mtproto (string: %s)", val.String())
		case reflect.Float32, reflect.Float64:
			return fmt.Sprintf("mtproto (float: %f)", val.Float())
		case reflect.Bool:
			return fmt.Sprintf("mtproto (bool: %t)", val.Bool())
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return fmt.Sprintf("mtproto (int: %d)", val.Int())
		default:
			return fmt.Sprintf("mtproto <type: %T>", target)
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
	ErrInvalidBase64PublicKey       = errors.New("the base64 public key is invalid, get and convert it from my.telegram.org")
	ErrInvalidRSAPublicKey          = errors.New("the public key is invalid, cannot find the RSA PEM block")
	ErrInvalidClient                = errors.New("cannot create a new MTProto API client, check your credentials and configuration")
	ErrInvalidClientCouldNotConnect = errors.New("could not connect to the MTProto MTProtoAPI, check your credentials and configuration")
)

// configuration errors
var (
	ErrOTPNotAccepted                = errors.New("the two-factor authentication code was not accepted")
	ErrInvalidConfiguration          = errors.New("the configuration is invalid, check your configuration")
	ErrInvalidNoChannelsFound        = errors.New("no channels were found, join the bot to a channel on MTProto and try again")
	ErrManagerFilesystemNotSupported = errors.New("the selected filesystem managers are not supported, check the remote names")
	ErrFilesystemsNotAvailable       = errors.New("some filesystem managers are not available, check they are written correctly")
)
