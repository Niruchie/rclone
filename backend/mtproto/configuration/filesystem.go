package configuration

import (
	"context"
	"fmt"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/rclone/rclone/backend/mtproto/configuration/hashing"
	"github.com/rclone/rclone/backend/mtproto/configuration/logging"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/hash"
)

// Wraps a filesystem with a mutex lock.
type ManagerWithLock struct {
	Fs   fs.Fs
	Lock sync.Mutex
}

// Filesystem with its properties.
type Filesystem struct {
	managers []ManagerWithLock
	hash     hash.Type
	name     string
	root     string
	MTProtoService
	fs.Fs
}

// Fs creates a new Filesystem instance with the specified parameters and required properties.
func Fs(ctx context.Context, name string, root string, m configmap.Mapper) (fs.Fs, error) {
	var err error = nil
	// ? Create a new Filesystem instance
	f := &Filesystem{
		MTProtoService: *NewMTProtoService(ctx),
	}

	// ? Parse the config into the struct
	err = configstruct.Set(m, &(f.Options))
	if err != nil {
		return nil, err
	}

	// ? Authorize the client into MTProto API.
	_, err = f.Authorize()
	if err != nil {
		return nil, err
	}

	// ? Register the hash types for the filesystem.
	size := hashing.NewTelegramMultipartHasher().Size()
	registeredType := hash.RegisterHash("telegramhashmulti", "TelegramMultipartHash", size, hashing.NewTelegramMultipartHasher)

	// ? Debugging the MTProto API connections
	client, err := f.Client()
	if err != nil {
		return nil, err
	}

	client.On(telegram.OnRaw, func(m telegram.Update, c *telegram.Client) error {
		fs.Debugf(logging.LoggerString(c), "Received raw update: %v", m)
		return nil
	})

	managers := []ManagerWithLock{}
	// ? Register the filesystem managers
	for _, manager := range f.Managers {
		bs, err := fs.NewFs(ctx, manager)
		if err != nil {
			return nil, err
		}
		managers = append(managers, ManagerWithLock{Fs: bs})
	}

	// ? Set up Filesystem instance
	f.hash = registeredType
	f.managers = managers
	f.root = root
	f.name = name
	return f, nil
}

// ? ----- Interface fs.Info -----

// Features returns the optional features of this Fs.
func (f *Filesystem) Features() *fs.Features {
	return NewMTProtoFeatures(f)
}

// Name of the remote (as passed into NewFs).
func (f *Filesystem) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs).
func (f *Filesystem) Root() string {
	root := clean(f.root)
	return root
}

// Returns the supported hash types of the filesystem.
func (f *Filesystem) Hashes() hash.Set {
	return hash.Set(f.hash)
}

// String returns a description of the filesystem.
func (f *Filesystem) String() string {
	return fmt.Sprintf("MTProto backend mounted at: %s:%s", f.name, f.root)
}

// Precision of the ModTimes in this filesystem.
func (f *Filesystem) Precision() time.Duration {
	mtproto, err := f.Client()
	if err != nil {
		return time.Second
	}

	return mtproto.Ping()
}

// ? ----- Path cleaning operations -----

// SlashOpCode represents the operation to perform on slashes.
type SlashOpCode string

const (
	UNTRAIL SlashOpCode = "untrail"
	UNLEAD  SlashOpCode = "unlead"
	TRAIL   SlashOpCode = "trail"
	LEAD    SlashOpCode = "lead"
)

// clean cleans the input path by removing leading and trailing slashes.
//
// Definition:
//
//	clean(input string) string
//
// Parameters:
//
//	input string - The input path to clean.
//
// Returns:
//
//	string - The cleaned path.
func clean(input string) string {
	input = slash(LEAD, input)
	input = path.Clean(input)
	return slash(UNTRAIL, input)
}

// slash modifies the input path by adding or removing leading and trailing slashes.
//
// Definition:
//
//	slash(op SlashOpCode, input string) string
//
// Parameters:
//
//	op SlashOpCode - The operation to perform (add/remove leading/trailing slash).
//	input string - The input path to modify.
//
// Returns:
//
//	string - The modified path.
func slash(op SlashOpCode, input string) string {
	switch op {
	case UNTRAIL:
		return strings.TrimSuffix(input, "/")
	case UNLEAD:
		return strings.TrimPrefix(input, "/")
	case TRAIL:
		if !strings.HasSuffix(input, "/") {
			input += "/"
		}
		return input
	case LEAD:
		if !strings.HasPrefix(input, "/") {
			return "/" + input
		}
		return input
	default:
		return input
	}
}
