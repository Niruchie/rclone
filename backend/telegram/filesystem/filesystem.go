package filesystem

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/rclone/rclone/backend/telegram/api"
	"github.com/rclone/rclone/backend/telegram/types"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/hash"

	"github.com/amarnathcjd/gogram/telegram"
)

type Filesystem struct {
	mtproto *telegram.Client
	bot     *telegram.Client
	name    string
	root    string
	types.Options
	fs.Fs
}

func Fs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	var err error

	// ? Create a new Filesystem instance
	f := &Filesystem{}

	// ? Parse the config into the struct
	err = configstruct.Set(m, &(f.Options))
	if err != nil {
		return nil, err
	}

	// ? Create a new Telegram MTProto instance
	mtproto, err := api.GetClientMTProto(
		f.AppId, f.AppHash,
		f.PublicKey,
		f.StringSession,
	)

	if err != nil {
		return nil, err
	}

	// ? Create a new Telegram Bot instance
	bot, err := api.GetClientBot(
		f.AppId, f.AppHash,
		f.PublicKey,
		f.BotToken,
	)

	if err != nil {
		return nil, err
	}

	// ? Debugging the Telegram API connections
	me, err := mtproto.GetMe()
	if err != nil {
		return nil, err
	}

	fs.LogPrintf(fs.LogLevelDebug, "Telegram MTProto API working with: ", me.Username)

	me, err = bot.GetMe()
	if err != nil {
		return nil, err
	}

	fs.LogPrintf(fs.LogLevelDebug, "Telegram Bot API working with: ", me.Username)

	// ? Set up Filesystem instance
	f.mtproto = mtproto
	f.root = root
	f.name = name
	f.bot = bot

	return f, nil
}

// ? ----- Interface fs.Info -----

func (f *Filesystem) Usage(ctx context.Context) (*fs.Usage, error) {
	return &fs.Usage{}, nil
}

func (f *Filesystem) Features() *fs.Features {
	return &fs.Features{
		ServerSideAcrossConfigs: false,
		CanHaveEmptyDirectories: false,
		CaseInsensitive:         false,
		DuplicateFiles:          false,
		FilterAware:             true,
		SlowModTime:             true,
		SlowHash:                true,
		Overlay:                 false,
		About:                   f.Usage,
	}
}

func (f *Filesystem) Name() string {
	return f.name
}

func (f *Filesystem) Root() string {
	root := path.Join("/root", Clean(f.root))
	root = UntrailSlash(path.Clean(root))
	return root
}

func (f *Filesystem) Hashes() hash.Set {
	return hash.Set(hash.SHA256)
}

func (f *Filesystem) String() string {
	return f.name + ":" + f.root
}

func (f *Filesystem) Precision() time.Duration {
	local := time.Now().Unix()

	server, err := f.mtproto.UpdatesGetState()
	if err != nil {
		return time.Second
	}

	remote := int64(server.Date) - local
	if remote <= 0 {
		return time.Second
	}

	return time.Duration(remote) * time.Second
}

// ? ----- Interface fs.Fs : fs.Info methods -----

func (f *Filesystem) List(ctx context.Context, relative string) (entries fs.DirEntries, err error) {
	// ? Get query with absolute path for topic.
	root := f.Root()
	relative = Clean(relative)
	absolute := path.Join(root, relative)
	query := UntrailSlash(absolute)
	fs.LogPrint(fs.LogLevelInfo, query)

	// ? Get the channel from API.
	channel, err := api.GetChannel(f.mtproto, f.ChannelId)
	if err != nil {
		return entries, err
	}

	// ? Get the topics from API.
	topics, err := api.GetTopics(f.mtproto, channel, query)
	if err != nil {
		return entries, err
	}

	// ? Iterate over the topics and create a new DirEntry for each.
	// * `topic.Title` must remove root trailed, due to path tree.
	// * `topic.Date` is int32, convert it to time.Time
	for _, topic := range topics {
		if path.Dir(topic.Title) == query {
			var items int64 = 0
			var offset int32 = 0
			trailRoot := TrailSlash(root)

			for {
				fs.LogPrintf(fs.LogLevelInfo, "Searching for file (as a Telegram Message): %s, topic: %s, topicId: %s, offset: %d", query, topic.Title, topic.ID, offset)
				_, amount, incomplete, next, err := api.SearchMessagesTopic(f.mtproto, channel, topic, query, offset)
				if err != nil {
					return entries, err
				}

				items += amount

				if incomplete && offset != next {
					offset = next
					continue
				}

				break
			}

			id := fmt.Sprintf("%d", topic.ID)
			name := strings.TrimPrefix(topic.Title, trailRoot)
			date := time.Unix(int64(topic.Date), 0)
			directory := fs.NewDir(name, date).SetID(id)

			// ? Directory attributes
			// * Unknown size, set to -1
			directory.SetItems(int64(topic.TopMessage))
			directory.SetRemote(name)
			directory.SetItems(items)
			directory.SetSize(-1)
			directory.SetID(id)

			entries = append(entries, directory)
		}
	}

	return entries, nil
}

func (f *Filesystem) Mkdir(ctx context.Context, relative string) error {
	// ? Get query with absolute path for topic.
	root := f.Root()
	relative = Clean(relative)
	absolute := path.Join(root, relative)
	query := UntrailSlash(absolute)

	// ? Get the channel from API.
	channel, err := api.GetChannel(f.mtproto, f.ChannelId)
	if err != nil {
		return err
	}

	// ? Create the topic on the channel.
	_, created, err := api.CreateTopic(f.mtproto, channel, query)
	if err == nil && !created {
		fs.LogPrintf(fs.LogLevelError, "Folder already exists (as a Telegram Topic): %s", query)
	} else if err == nil {
		fs.LogPrintf(fs.LogLevelInfo, "Folder created (as a Telegram Topic): %s", query)
	} else {
		fs.LogPrintf(fs.LogLevelError, "Error creating folder (as a Telegram Topic): %s", query)
		return err
	}

	return nil
}

func (f *Filesystem) Rmdir(ctx context.Context, relative string) error {
	// ? Get query with absolute path for topic.
	root := f.Root()
	relative = Clean(relative)
	absolute := path.Join(root, relative)
	query := UntrailSlash(absolute)

	// ? Get the channel from API.
	channel, err := api.GetChannel(f.mtproto, f.ChannelId)
	if err != nil {
		return err
	}

	// ? Get the topics from API.
	topics, err := api.GetTopics(f.mtproto, channel, query)
	if err != nil {
		return err
	}

	amount := len(topics)
	if amount == 0 {
		return types.ErrDirectoryNotFound
	}

	// ? Iterate the topics searching the topic to delete.
	var found *telegram.ForumTopicObj = nil
	var offset int32 = 0
	var count uint64 = 0

	for _, topic := range topics {
		if topic.Title == query {
			if amount > 1 {
				fs.LogPrintf(fs.LogLevelError, "Error deleting folder (as a Telegram Topic): %s, subfolders were found", query)
				return types.ErrDirectoryNotRemoved
			}

			found = topic
			break
		}
	}

	if found == nil {
		fs.LogPrintf(fs.LogLevelError, "Error deleting folder (as a Telegram Topic): %s, folder not found", query)
		return types.ErrDirectoryNotFound
	}

	for {
		fs.LogPrintf(fs.LogLevelInfo, "Searching for file (as a Telegram Message): %s, topic: %s, topicId: %s", query, found.Title, found.ID)
		_, items, incomplete, next, err := api.SearchMessagesTopic(f.mtproto, channel, found, query, offset)
		if err != nil {
			return err
		}

		count += uint64(items)

		if count > 1 { // ? Exclude the service message from Telegram API.
			fs.LogPrintf(fs.LogLevelError, "Error deleting folder (as a Telegram Topic): %s, folder is not empty", query)
			return types.ErrDirectoryNotRemoved
		}

		// ? If the search is incomplete, continue searching.
		if incomplete && offset != next {
			continue
		}

		break
	}

	// ? Delete the topic if it's empty.
	err = api.DeleteTopic(f.mtproto, channel, found)
	if err != nil {
		fs.LogPrintf(fs.LogLevelError, "Error deleting folder (as a Telegram Topic): %s, %s", query, err.Error())
		return types.ErrDirectoryNotRemoved
	}

	return nil
}

func (f *Filesystem) NewObject(ctx context.Context, relative string) (fs.Object, error) {
	// ? Get query with absolute path for topic.
	root := f.Root()
	relative = Clean(relative)
	absolute := path.Join(root, relative)
	query := UntrailSlash(absolute)
	fs.LogPrint(fs.LogLevelInfo, query)

	// ? Get the channel from API.
	channel, err := api.GetChannel(f.mtproto, f.ChannelId)
	if err != nil {
		return nil, err
	}

	// ? Get the topics from API.
	// * Search for dir containing topic, to search for files next.
	topics, err := api.GetTopics(f.mtproto, channel, path.Dir(query))
	if err != nil {
		return nil, err
	}

	var dirTopic *telegram.ForumTopicObj = nil
	var offset int32 = 0

	// * If remote points to a directory then
	// * -- fs.ErrorIsDir should be returned.
	for _, topic := range topics {
		if topic.Title == query {
			return nil, fs.ErrorIsDir
		}

		if topic.Title == path.Dir(query) {
			dirTopic = topic
		}
	}

	// ? Impossible to search for files without a topic.
	// * If it can't be found it returns the error ErrorObjectNotFound.
	if dirTopic == nil {
		fs.LogPrintf(fs.LogLevelError, "Error searching for file (as a Telegram Message): %s, topic not found as a Telegram Topic", query)
		return nil, fs.ErrorObjectNotFound
	}

	for {
		fs.LogPrintf(fs.LogLevelInfo, "Searching for file (as a Telegram Message): %s, topic: %s, topicId: %s, offset: %d", query, dirTopic.Title, dirTopic.ID, offset)
		items, _, incomplete, next, err := api.SearchMessagesTopic(f.mtproto, channel, dirTopic, query, offset)
		if err != nil {
			return nil, err
		}

		for _, item := range items {
			switch found := item.(type) {
			case *telegram.MessageObj:
				if found.Message == query {
					fs.LogPrintf(fs.LogLevelInfo, "File found (as a Telegram Message): %s, offset: %d", query, offset)
					return &Object{
						absolute:   absolute,
						relative:   relative,
						filesystem: f,
					}, nil
				}
			// ? Skip service messages and ignore other types.
			case *telegram.MessageService:
			default:
				continue
			}
		}

		if incomplete && offset != next {
			offset = next
			continue
		}

		return nil, fs.ErrorObjectNotFound
	}
}

func (f *Filesystem) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return nil, types.ErrUnsupportedOperation
}
