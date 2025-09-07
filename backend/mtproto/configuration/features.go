package configuration

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/rclone/rclone/backend/mtproto/configuration/logging"
	"github.com/rclone/rclone/fs"
)

// NewMTProtoFeatures creates a new feature set for the backend.
func NewMTProtoFeatures(f *Filesystem) *fs.Features {
	return &fs.Features{
		CaseInsensitive:          false,
		DuplicateFiles:           false,
		ReadMimeType:             false, // TODO
		WriteMimeType:            false,
		CanHaveEmptyDirectories:  true,
		BucketBased:              true,
		BucketBasedRootOK:        true,
		SetTier:                  false, // Unknown
		GetTier:                  false, // Unknown
		ServerSideAcrossConfigs:  false, // TODO
		IsLocal:                  false,
		SlowModTime:              true,
		SlowHash:                 true,
		ReadMetadata:             true,
		WriteMetadata:            false,
		UserMetadata:             true,
		ReadDirMetadata:          true,
		WriteDirMetadata:         false,
		WriteDirSetModTime:       false,
		UserDirMetadata:          false,
		DirModTimeUpdatesOnWrite: false, // TODO
		FilterAware:              true,
		PartialUploads:           false, // TODO
		NoMultiThreading:         false,
		Overlay:                  false, // Unknown
		ChunkWriterDoesntSeek:    false,

		// ? ----- Implements the following methods -----
		About:        f.About,
		ChangeNotify: f.ChangeNotify,
		DirMove:      f.DirMove,
	}
}

// About gets the quota information for the Fs.
//
// Parameters:
//
//	ctx context.Context - The context for the request.
//
// Returns:
//
//	*fs.Usage - The usage information.
//	error - If an error occurred.
func (f *Filesystem) About(ctx context.Context) (*fs.Usage, error) {
	return &fs.Usage{}, nil
}

// DirMove moves src, srcRemote to this remote at dstRemote
// using server-side move operations.
//
// Read more about the method at [Features.DirMove].
//
// [Features.DirMove]: https://pkg.go.dev/github.com/rclone/rclone/fs#Features.DirMove
func (f *Filesystem) DirMove(ctx context.Context, from fs.Fs, src string, dst string) error {
	destination := telegram.ForumTopicObj{ID: 0, Title: dst}
	source := telegram.ForumTopicObj{ID: 0, Title: src}

	if list, err := f.GetTopics(ctx, destination); err == nil {
		for _, forumTopic := range list {
			if forumTopic.Title == destination.Title {
				return fs.ErrorDirExists
			}
		}
	} else {
		return fs.ErrorCantDirMove
	}

	if list, err := f.GetTopics(ctx, source); err == nil {
		for _, forumTopic := range list {
			if forumTopic.Title == source.Title {
				source = forumTopic
			}
		}
	} else {
		return fs.ErrorCantDirMove
	}

	if source.ID == 0 {
		return fs.ErrorCantDirMove
	}

	_, updated, err := f.UpdateTopic(ctx, source)
	switch {
	case err != nil:
		return fs.ErrorCantDirMove
	case !updated:
		return fs.ErrorCantDirMove
	default:
		return nil
	}
}

// Fetch updates from MTProto API and handle using the given function.
//
// Parameters:
//
//	pts int32 - The current pts value.
//	handle func(string, fs.EntryType) - The function to call with the path that has had changes.
//
// Returns:
//
//	next time.Time - The next time to poll.
//	update bool - Whether there are updates to process.
//	seq int32 - The next pts value.
//	err error - If an error occurred.
func (f *Filesystem) fetchUpdates(ptsIn int32, handle func(string, fs.EntryType)) (next time.Time, update bool, pts int32, err error) {
	next, update, pts, err = time.Now(), false, ptsIn, nil
	log := "invoking supergroup forum updates"
	fs.Debugf(logging.LoggerString(f), log)

	client, err := f.Client()
	if err != nil {
		return next, update, pts, err
	}

	channel, err := client.GetChannel(f.SupergroupId)
	if err != nil {
		return next, update, pts, err
	}

	input := &telegram.InputChannelObj{
		AccessHash: channel.AccessHash,
		ChannelID:  channel.ID,
	}

	// TODO: check performance with Limit
	var limited int32 = 50
	request := &telegram.UpdatesGetChannelDifferenceParams{
		Filter:  &telegram.ChannelMessagesFilterEmpty{},
		Limit:   limited,
		Channel: input,
		Pts:     ptsIn,
		Force:   true,
	}

	diff, err := client.UpdatesGetChannelDifference(request)
	if err != nil {
		return next, update, pts, err
	}

	redirectToHandler := func(messages []telegram.Message, chats []telegram.Chat, users []telegram.User) {
		for _, msg := range messages {
			switch message := msg.(type) {
			case *telegram.MessageService:
				root := f.Root()
				switch message.Action.(type) {
				case *telegram.MessageActionTopicCreate, *telegram.MessageActionTopicEdit:
					forumTopics, _ := client.ChannelsGetForumTopicsByID(input, []int32{message.ID})
					for _, forumTopic := range forumTopics.Topics {
						switch forumTopic := forumTopic.(type) {
						case *telegram.ForumTopicObj:
							if strings.HasPrefix(forumTopic.Title, root) {
								handle(forumTopic.Title, fs.EntryDirectory)
								log := "detected new forum topic directory %q"
								fs.Debugf(logging.LoggerString(f), log, forumTopic.Title)
							}
						}
					}
				}
			}
		}
	}

	switch diff := diff.(type) {
	case *telegram.UpdatesChannelDifferenceEmpty:
		next = time.Now().Add(time.Duration(diff.Timeout) * time.Second)
		update = diff.Final && false // next polling only
		pts = diff.Pts

		log := "no difference, current pts=%d, next pts=%d, no timeout=%d, should update=%t"
		fs.Debugf(logging.LoggerString(f), log, ptsIn, pts, diff.Timeout, update)
	case *telegram.UpdatesChannelDifferenceObj:
		next = time.Now().Add(time.Duration(diff.Timeout) * time.Second)
		update = diff.Final
		pts = diff.Pts

		log := "difference found (short), no re-sync, current pts=%d, next pts=%d, timeout=%d, should update=%t"
		fs.Debugf(logging.LoggerString(f), log, pts, ptsIn, diff.Timeout, update)
		redirectToHandler(diff.NewMessages, diff.Chats, diff.Users)
	case *telegram.UpdatesChannelDifferenceTooLong:
		next = time.Now().Add(time.Duration(diff.Timeout) * time.Second)
		update = diff.Final
		pts = ptsIn + limited

		log := "difference found (long), need to re-sync, current pts=%d, next pts=%d, timeout=%d, should update=%t"
		fs.Debugf(logging.LoggerString(f), log, ptsIn, pts, diff.Timeout, update)
		redirectToHandler(diff.Messages, diff.Chats, diff.Users)
	}

	return next, update, pts, err
}

// Fetch updates and schedule next poll if needed.
//
// Parameters:
//
//	pts int32 - The current pts value.
//	updates chan<- time.Time - The channel to send the next poll time to.
//	handle func(string, fs.EntryType) - The function to call with the path that has had changes.
//
// Returns:
//
//	int32 - The next pts value.
//	error - If an error occurred.
func (f *Filesystem) polling(pts int32, updates chan<- time.Time, handle func(string, fs.EntryType)) (int32, error) {
	if next, schedule, pts, err := f.fetchUpdates(pts, handle); err == nil {
		if schedule {
			select {
			case updates <- next:
				log := "scheduling next poll at %v"
				fs.Debugf(logging.LoggerString(f), log, next)
			default:
			}
		}

		return pts, err
	} else {
		return pts, err
	}
}

// ChangeNotify calls the passed function with a path that has had changes.
// If the implementation uses polling, it should adhere to the given interval.
//
// Read more about the method at [Features.ChangeNotify].
//
// [Features.ChangeNotify]: https://pkg.go.dev/github.com/rclone/rclone/fs#Features.ChangeNotify
func (f *Filesystem) ChangeNotify(ctx context.Context, handleEntry func(string, fs.EntryType), intervals <-chan time.Duration) {
	handleChangeNotifications := func() {
		var client *telegram.Client
		var pts int32 = 0
		var err error

		client, err = f.Client()
		if err != nil {
			log := "error creating polling client, %s"
			fs.Errorf(logging.LoggerString(f), log, err.Error())
			return
		}

		channel, err := client.GetChannel(f.SupergroupId)
		if err != nil {
			log := "error fetching polling supergroup forum, %s"
			fs.Errorf(logging.LoggerString(f), log, err.Error())
			return
		}

		input := &telegram.InputChannelObj{
			AccessHash: channel.AccessHash,
			ChannelID:  channel.ID,
		}

		response, err := client.ChannelsGetFullChannel(input)
		if err != nil {
			log := "error fetching polling supergroup forum, %s"
			fs.Errorf(logging.LoggerString(f), log, err.Error())
			return
		}

		channelFull, ok := response.FullChat.(*telegram.ChannelFull)
		if !ok {
			log := "error fetching polling supergroup forum, not a channel"
			fs.Errorf(logging.LoggerString(f), log)
			return
		}

		ticker := time.NewTicker(time.Minute)
		updates := make(chan time.Time, 1)
		ptsMutexLock := sync.Mutex{}
		pts = channelFull.Pts
		ticks := ticker.C

		for {
			select {
			case interval, ok := <-intervals:
				switch {
				case !ok:
					log := "ticking interval not received"
					fs.Debugf(logging.LoggerString(f), log)
				case ticker != nil:
					ticker.Stop()
					ticker, ticks = nil, nil
				case interval != 0:
					ticker = time.NewTicker(interval)
					updates <- time.Now()
					ticks = ticker.C
				}
			case <-ticks:
				locked := ptsMutexLock.TryLock()
				if !locked {
					log := "skipping tick, previous tick is still being processed"
					fs.Debugf(logging.LoggerString(f), log)
					continue
				}

				next, err := f.polling(pts, updates, handleEntry)
				if err != nil {
					log := "error polling updates, %s"
					fs.Errorf(logging.LoggerString(f), log, err.Error())
				}

				pts = next
				ptsMutexLock.Unlock()
			case u := <-updates:
				until := time.Until(u)
				log := "waiting for next poll at %v"
				fs.Debugf(logging.LoggerString(f), log, u)

				ptsMutexLock.Lock()
				time.AfterFunc(until, func() {
					next, err := f.polling(pts, updates, handleEntry)
					if err != nil {
						log := "error polling updates, %s"
						fs.Errorf(logging.LoggerString(f), log, err.Error())
					}
					pts = next
				})
				ptsMutexLock.Unlock()
			case <-ctx.Done():
				log := "shutting down change notification handler"
				fs.Debugf(logging.LoggerString(f), log)
				return
			}
		}
	}
	go handleChangeNotifications()
}
