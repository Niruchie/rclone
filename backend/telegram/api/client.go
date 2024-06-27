package api

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/rclone/rclone/backend/telegram/types"
	"github.com/rclone/rclone/fs"
)

// ? Get the client from the Telegram MTProto API.
func GetClientMTProto(AppId int32, AppHash string, PublicKey string, StringSession string) (*telegram.Client, error) {
	// ? Decode the public key.
	decoded, err := base64.StdEncoding.DecodeString(PublicKey)
	if err != nil {
		return nil, types.ErrInvalidBase64PublicKey
	}

	// ? Decode the PEM block.
	block, _ := pem.Decode([]byte(decoded))
	key, err := x509.ParsePKCS1PublicKey(block.Bytes)
	if err != nil {
		return nil, types.ErrInvalidRSAPublicKey
	}

	keys := []*rsa.PublicKey{key}

	// ? The App ID and App Hash are used to authenticate the client.
	client, err := telegram.NewClient(telegram.ClientConfig{
		DeviceConfig: telegram.DeviceConfig{
			DeviceModel:   fmt.Sprintf("rclone %s %s", fs.VersionTag, fs.VersionSuffix),
			SystemVersion: fs.VersionSuffix,
			AppVersion:    fs.VersionTag,
		},
		LogLevel:      telegram.LogError,
		TestMode:      false,
		MemorySession: true,
		DisableCache:  true,
		LangCode:      "en",

		StringSession: StringSession,
		AppHash:       AppHash,
		AppID:         AppId,
		PublicKeys:    keys,
	})

	if err != nil {
		return nil, types.ErrInvalidClient
	}

	// ? Connect the client to the Telegram MTProto API.
	err = client.Connect()
	if err != nil {
		return nil, types.ErrInvalidClientCouldNotConnect
	}

	return client, err
}

// ? Get the client from the Telegram Bot API.
func GetClientBot(AppId int32, AppHash string, PublicKey string, BotToken string) (*telegram.Client, error) {
	// ? Get an MTProto client with empty string session.
	client, err := GetClientMTProto(AppId, AppHash, PublicKey, types.SessionStringEmpty)
	if err != nil {
		return nil, err
	}

	// ? Connect the client to the Telegram Bot API.
	err = client.ConnectBot(BotToken)
	if err != nil {
		return nil, types.ErrInvalidClientCouldNotConnectBot
	}

	return client, err
}
