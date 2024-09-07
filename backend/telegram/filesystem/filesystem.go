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
	hash hash.Type
	name string
	root string
	api.TelegramClient
	fs.Fs
}

func Fs(ctx context.Context, name string, root string, m configmap.Mapper) (fs.Fs, error) {
	var log string = ""
	var err error = nil

	// ? Create a new Filesystem instance
	f := &Filesystem{}

	// ? Parse the config into the struct
	err = configstruct.Set(m, &(f.Options))
	if err != nil {
		return nil, err
	}

	// ? Create a new Telegram API connection
	mtproto, bot, err := f.Connect(ctx)
	if err != nil {
		return nil, err
	}

	// ? Register the hash types for the filesystem.
	size := types.NewTelegramMultipartHasher().Size()
	registeredType := hash.RegisterHash("telegramhashmulti", "TelegramMultipartHash", size, types.NewTelegramMultipartHasher)

	// ? Debugging the Telegram API connections
	me, err := mtproto.GetMe()
	if err != nil {
		return nil, err
	}

	log = fmt.Sprintf("Telegram MTProto API working with: %s", me.Username)
	fs.LogPrint(fs.LogLevelInfo, log)

	me, err = bot.GetMe()
	if err != nil {
		return nil, err
	}

	log = fmt.Sprintf("Telegram Bot API working with: %s", me.Username)
	fs.LogPrint(fs.LogLevelInfo, log)

	// ? Set up Filesystem instance
	f.hash = registeredType
	f.root = root
	f.name = name

	return f, nil
}

// Returns the root, relative and query path for the filesystem.
//
// Definition:
//
//	Locate(relative string) (string, string, string)
//
// Parameters:
//
//	relative - The relative path to search for the entry.
//
// Returns:
//
//	root - The root path of the filesystem.
//	relative - The relative path of the entry.
//	query - The query path of the entry.
func (f *Filesystem) Locate(relative string) (string, string, string) {
	root := f.Root()
	absolute := path.Join(root, relative)
	query := UntrailSlash(absolute)

	log := fmt.Sprintf("Locate query for entry -> Absolute: %s, Relative: %s, Query: %s", absolute, relative, query)
	fs.LogPrint(fs.LogLevelDebug, log)

	return root, relative, query
}

// Returns all the directories from the directory filesystem tree.
//
// Definition:
//
//	Directory(query string) (*ForumTopicObj, error)
//
// Parameters:
//
//	query - The query to search for the directory. | File absolute path.
//
// Error is handled by the callee.
func (f *Filesystem) Directory(ctx context.Context, query string) (*telegram.ForumTopicObj, error) {
	topics, err := f.GetTopics(ctx, query)
	if err != nil {
		return nil, err
	}

	var dirTopic *telegram.ForumTopicObj = nil
	for _, topic := range topics {
		if topic.Title == query {
			dirTopic = topic
		}
	}

	if dirTopic == nil {
		return nil, fs.ErrorDirNotFound
	}

	return dirTopic, nil
}

// Returns all the directories from the directory filesystem tree.
//   - It's better to use this method than calling for topics directly.
//
// Definition:
//
//	Directories() ([]*ForumTopicObj, error)
//
// Error is handled by the callee.
func (f *Filesystem) Directories(ctx context.Context) ([]*telegram.ForumTopicObj, error) {
	return f.GetTopics(ctx, f.Root())
}

// Returns the directories from the directory passed.
//
// Definition:
//
//	DirectoriesFrom(topic *ForumTopicObj) ([]*ForumTopicObj, error)
//
// Parameters:
//
//	topic - The topic to search for the objects. | Get it from Directory() method.
//
// Error is handled by the callee.
func (f *Filesystem) DirectoriesFrom(ctx context.Context, topic *telegram.ForumTopicObj) ([]*telegram.ForumTopicObj, error) {
	topics, err := f.GetTopics(ctx, topic.Title)
	if err != nil {
		return topics, err
	}

	var children []*telegram.ForumTopicObj = nil
	for _, subtopic := range topics {
		if path.Dir(subtopic.Title) == topic.Title {
			children = append(children, subtopic)
		}
	}

	return children, nil
}

// Returns the objects in the directory passed.
//
// Definition:
//
//	Objects(topic *ForumTopicObj) ([]*Object, int64, error)
//
// Parameters:
//
//	topic - The topic to search for the objects. | Get it from Directory() method.
//
// Error is handled by the callee.
func (f *Filesystem) Objects(ctx context.Context, topic *telegram.ForumTopicObj) ([]*Object, int64, error) {
	objects := make([]*Object, 0)
	var offset int32 = 0
	var items int64 = 0

	for {
		log := fmt.Sprintf("Searching for objects (as Telegram Messages) on topic: %s, topicId: %d, offset: %d", topic.Title, topic.ID, offset)
		fs.LogPrint(fs.LogLevelDebug, log)

		messages, amount, incomplete, next, err := f.SearchMessagesTopic(ctx, topic, topic.Title, offset)
		if err != nil {
			return objects, items, err
		}

		items += int64(amount)
		for _, message := range messages {
			switch found := message.(type) {
			case *telegram.MessageObj:
				if path.Dir(found.Message) == topic.Title {
					log := fmt.Sprintf("Object found (as Telegram Message): %s, offset: %d, id: %d", found.Message, offset, found.ID)
					fs.LogPrint(fs.LogLevelDebug, log)
					object := NewObject(f, found)
					objects = append(objects, &object)
					continue
				}

			// ? Skip service messages and ignore other types.
			default:
				items--
				continue
			}
		}

		if incomplete && offset != next {
			offset = next
			continue
		}

		return objects, items, nil
	}
}

// Searches for an object in the filesystem.
//
// Definition:
//
//	ObjectSearch(topic *ForumTopicObj, query string) (*Object, error)
//
// Parameters:
//
//	topic - The topic to search for the object. | Get it from Directory() method.
//	query - The query to search for the object. | File absolute path.
//
// Error is handled by the callee.
func (f *Filesystem) ObjectSearch(ctx context.Context, topic *telegram.ForumTopicObj, query string) (*Object, error) {
	var offset int32 = 0

	for {
		log := fmt.Sprintf("Searching for object (as Telegram Message): %s, topic: %s, topicId: %d, offset: %d", query, topic.Title, topic.ID, offset)
		fs.LogPrint(fs.LogLevelDebug, log)

		messages, _, incomplete, next, err := f.SearchMessagesTopic(ctx, topic, query, offset)
		if err != nil {
			return nil, err
		}

		for _, message := range messages {
			switch found := message.(type) {
			case *telegram.MessageObj:
				if found.Message == query {
					log := fmt.Sprintf("Object found (as Telegram Message): %s, offset: %d, id: %d", query, offset, found.ID)
					fs.LogPrint(fs.LogLevelDebug, log)
					object := NewObject(f, found)
					return &object, nil
				}
				continue

			// ? Skip service messages and ignore other types.
			case *telegram.MessageService:
			default:
				log := fmt.Sprintf("Ignoring object (as Telegram Message): %s, offset: %d, type: %T", query, offset, found)
				fs.LogPrint(fs.LogLevelDebug, log)
				continue
			}
		}

		if incomplete && offset != next {
			offset = next
			continue
		}

		log = fmt.Sprintf("Object not found (as Telegram Message): %s", query)
		fs.LogPrint(fs.LogLevelDebug, log)
		return nil, fs.ErrorObjectNotFound
	}
}

// ? ----- Interface fs.Info -----

// Features returns the optional features of this Fs.
func (f *Filesystem) Features() *fs.Features {
	return NewTelegramFeatures(f)
}

// Name of the remote (as passed into NewFs).
func (f *Filesystem) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs).
func (f *Filesystem) Root() string {
	root := path.Join("/root", Clean(f.root))
	root = UntrailSlash(path.Clean(root))
	return root
}

// Returns the supported hash types of the filesystem.
func (f *Filesystem) Hashes() hash.Set {
	return hash.Set(f.hash)
}

// String returns a description of the filesystem.
func (f *Filesystem) String() string {
	return fmt.Sprintf("Telegram backend mounted at: %s:%s", f.name, f.root)
}

// Precision of the ModTimes in this filesystem.
func (f *Filesystem) Precision() time.Duration {
	mtproto, err := f.MTProto()
	if err != nil {
		return time.Second
	}

	return mtproto.Ping()
}

// ? ----- Interface fs.Fs : fs.Info methods -----

// List the objects and directories in dir into entries.
//
// Read more about the method at [Fs.List]
//
// [Fs.List]: https://pkg.go.dev/github.com/rclone/rclone/fs#Fs.List
func (f *Filesystem) List(ctx context.Context, relative string) (entries fs.DirEntries, err error) {
	// ? Get locate query for entry.
	root, _, query := f.Locate(relative)

	// ? Get the directory topic from filesystem.
	topic, err := f.Directory(ctx, query)
	if err != nil {
		if relative == "" {
			dir := path.Dir(query)
			topic, err := f.Directory(ctx, dir)
			if err != nil {
				return nil, fs.ErrorDirNotFound
			}

			// ? Get the child directory topics from filesystem.
			object, err := f.ObjectSearch(ctx, topic, query)
			if err != nil {
				return nil, fs.ErrorDirNotFound
			}

			entries = append(entries, object)
			return entries, nil
		}

		return entries, fs.ErrorListAborted
	}

	// ? Get the child directory topics from filesystem.
	topics, err := f.DirectoriesFrom(ctx, topic)
	if err != nil {
		return entries, fs.ErrorListAborted
	}

	// ? Iterate over the topics and create a new DirEntry for each.
	// * `topic.Title` must remove root trailed, due to path tree.
	// * `topic.Date` is int32, convert it to time.Time
	for _, subtopic := range topics {
		trailRoot := TrailSlash(root)

		_, items, err := f.Objects(ctx, subtopic)

		// !!! Error handling for the objects in the folder.
		if err != nil {
			log := fmt.Sprintf("Error getting objects from folder (on Telegram Topic): %s, %s", query, err.Error())
			fs.LogPrint(fs.LogLevelError, log)
		}

		name := strings.TrimPrefix(subtopic.Title, trailRoot)
		date := time.Unix(int64(subtopic.Date), 0)
		id := fmt.Sprintf("%d", subtopic.ID)

		// ? Directory attributes
		// * Unknown size, set to -1
		directory := fs.NewDir(name, date)
		directory.SetRemote(name)
		directory.SetItems(items)
		directory.SetSize(-1)
		directory.SetID(id)

		entries = append(entries, directory)
	}

	// ? Get the objects from the directory topic.
	objects, _, err := f.Objects(ctx, topic)
	if err != nil {
		return entries, fs.ErrorListAborted
	}

	// ? Iterate and add each object to the entries.
	for _, object := range objects {
		entries = append(entries, object)
	}

	return entries, nil
}

// Mkdir makes the directory.
// Shouldn't return an error if it already exists.
func (f *Filesystem) Mkdir(ctx context.Context, relative string) error {
	// ? Get locate query for entry.
	_, _, query := f.Locate(relative)

	// ? Create the topic on the channel.
	log := fmt.Sprintf("Creating folder (as a Telegram Topic): %s", query)
	fs.LogPrint(fs.LogLevelDebug, log)

	_, created, err := f.CreateTopic(ctx, query)
	if err == nil && !created {

		log := fmt.Sprintf("Folder already exists (as a Telegram Topic): %s", query)
		fs.LogPrint(fs.LogLevelInfo, log)
		return nil

	} else if err == nil {

		log := fmt.Sprintf("Folder created (as a Telegram Topic): %s", query)
		fs.LogPrint(fs.LogLevelInfo, log)
		return nil

	} else {

		log := fmt.Sprintf("Error creating folder (as a Telegram Topic): %s, %s", query, err.Error())
		fs.LogPrint(fs.LogLevelError, log)
		return fs.ErrorDirNotFound

	}
}

// Rmdir removes the directory if empty.
// Return an error if it doesn't exist or isn't empty.
func (f *Filesystem) Rmdir(ctx context.Context, relative string) error {
	// ? Get locate query for entry.
	_, _, query := f.Locate(relative)

	// ? Searching the directory topic to delete.
	topic, err := f.Directory(ctx, query)
	if err != nil {
		return fs.ErrorDirNotFound
	}

	// ? Searching the child directory topics.
	children, err := f.DirectoriesFrom(ctx, topic)
	if err != nil {
		return fs.ErrorNotDeletingDirs
	}

	if len(children) > 0 {
		log := fmt.Sprintf("Error deleting folder (as Telegram Topic): %s, folder is not empty", query)
		fs.LogPrint(fs.LogLevelError, log)
		return fs.ErrorDirectoryNotEmpty
	}

	// ? Searching for child object on directory topic.
	// * Message service is ignored from the list.
	_, items, err := f.Objects(ctx, topic)
	if err != nil {
		return fs.ErrorNotDeletingDirs
	}

	if items > 0 {
		log := fmt.Sprintf("Error deleting folder (as Telegram Topic): %s, folder is not empty", query)
		fs.LogPrint(fs.LogLevelError, log)
		return fs.ErrorDirectoryNotEmpty
	}

	// ? Delete the topic if it's empty.
	err = f.DeleteTopic(ctx, topic)
	if err != nil {
		log := fmt.Sprintf("Error deleting folder (as Telegram Topic): %s, %s", query, err.Error())
		fs.LogPrint(fs.LogLevelError, log)
		return fs.ErrorNotDeletingDirs
	}

	return nil
}

// NewObject finds the Object at remote.
//
// Read more about the method at [Fs.NewObject]
//
// [Fs.NewObject]: https://pkg.go.dev/github.com/rclone/rclone/fs#Fs.NewObject
func (f *Filesystem) NewObject(ctx context.Context, relative string) (fs.Object, error) {
	// ? Get locate query for entry.
	_, _, query := f.Locate(relative)

	// ? Find the topic containing the file if any.
	// * Impossible to search for files without a topic.
	// * If it can't be found it returns the error ErrorObjectNotFound.
	directoryName := path.Dir(query)
	topic, err := f.Directory(ctx, directoryName)
	if err != nil {
		return nil, fs.ErrorDirNotFound
	}

	// ? Get the child directory topics from filesystem.
	// * Search for dir containing topic, to search for files next.
	if topics, err := f.DirectoriesFrom(ctx, topic); err == nil {
		// * If remote points to a directory then
		// * -- fs.ErrorIsDir should be returned.
		for _, topic := range topics {
			if topic.Title == query {
				return nil, fs.ErrorIsDir
			}
		}
	} else {
		return nil, fs.ErrorDirNotFound
	}

	return f.ObjectSearch(ctx, topic, query)
}

// Put in to the remote path with the modTime given of the given size
//
// Read more about the method at [Fs.Put]
//
// [Fs.Put]: https://pkg.go.dev/github.com/rclone/rclone/fs#Fs.Put
func (f *Filesystem) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	log := fmt.Sprintf("Put: %s Size: %d ModTime: %v", src.Remote(), src.Size(), src.ModTime(ctx))
	fs.LogPrint(fs.LogLevelInfo, log)

	o := NewObjectFromRelative(f, src.Remote())
	return &o, o.Update(ctx, in, src, options...)
}
