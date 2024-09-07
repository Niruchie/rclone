package types

import (
	"bytes"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/json"
	"hash"
	"io"
	"sort"

	"github.com/amarnathcjd/gogram/telegram"
)

var TelegramMultipartHasherPartSize int64 = 131072

type TelegramMultipartHash struct {
	sha256 []*telegram.FileHash
	data   []byte
	hash.Hash
	io.Writer
}

// NewTelegramMultipartHasher will generate a hasher for the filesystem.
//
// Definition:
//
//	NewTelegramMultipartHasher() hash.Hash
//
// Returns:
//
//	hash.Hash - The hasher for the filesystem.
func NewTelegramMultipartHasher() hash.Hash {
	return &TelegramMultipartHash{
		sha256: []*telegram.FileHash{},
		data:   []byte{},
	}
}

// NewTelegramMultipartHash will generate a hash from the list of hashes provided for each chunk.
// Commonly used by the objects after the uploads are complete or streamed.
//
// Definition:
//
//	NewTelegramMultipartHash(hashes []*telegram.FileHash, server bool) []byte
//
// Returns:
//
//	[]byte - The hash of the file.
func NewTelegramMultipartHash(hashes []*telegram.FileHash, server bool) []byte {
	hasher := &TelegramMultipartHash{
		sha256: []*telegram.FileHash{},
		data:   []byte{},
	}

	return hasher.FromFileHash(hashes, server)
}

// ? ----- Interface TelegramMultipartHash -----

// FromFileHash will generate a hash from the list of hashes provided for each chunk.
//
// Definition:
//
//	FromFileHash(hashes []*telegram.FileHash, server bool) []byte
//
// Returns:
//
//	[]byte - The hash of the file.
func (t *TelegramMultipartHash) FromFileHash(hashes []*telegram.FileHash, server bool) []byte {
	// ? Order the hashes by offset in place.
	sort.SliceStable(hashes, func(i, j int) bool {
		return hashes[i].Offset < hashes[j].Offset
	})
	
	// ? Remove duplicate offsets from the list.
	unique := hashes[:0]
	for i, hash := range hashes {
		if i == 0 || hash.Offset != hashes[i-1].Offset {
			unique = append(unique, hash)
		}
	}

	// ? Convert the list into raw data.
	data, err := json.Marshal(unique)
	if err != nil {
		return data
	}

	h := sha512.New()
	h.Write(data)
	return h.Sum(nil)
}

// ? ----- Interface hash.Hash -----

// Sum appends the current hash to b and returns the resulting slice.
//   - Inherited from the [hash.Hash] interface.
//
// [hash.Hash]: https://golang.org/pkg/hash/#Hash
func (t *TelegramMultipartHash) Sum(b []byte) []byte {
	// ? Convert the data into a io.ReaderSeeker
	reader := bytes.NewReader(t.data)
	var size int32 = 131072
	var offset int64 = 0

	for offset < int64(len(t.data)) {
		reader.Seek(offset, io.SeekStart)
		chunk := make([]byte, size)

		// ? Read from the offset, size amount of bytes
		n, err := reader.Read(chunk)
		if err != nil {
			return nil
		}

		if n < int(size) {
			chunk = chunk[:n]
		}

		hasher := sha256.New()
		hasher.Write(chunk)
		h := hasher.Sum(nil)

		t.sha256 = append(t.sha256, &telegram.FileHash{
			Offset: offset,
			Limit:  size,
			Hash:   h,
		})

		offset += int64(size)
	}

	return t.FromFileHash(t.sha256, false)
}

// Reset resets the Hash to its initial state.
//   - Inherited from the [hash.Hash] interface.
//
// [hash.Hash]: https://golang.org/pkg/hash/#Hash
func (t *TelegramMultipartHash) Reset() {
	t.sha256 = []*telegram.FileHash{}
	t.data = []byte{}
}

// Size returns the number of bytes Sum will return.
//   - Inherited from the [hash.Hash] interface.
//
// [hash.Hash]: https://golang.org/pkg/hash/#Hash
func (t *TelegramMultipartHash) Size() int {
	return sha512.Size
}

// BlockSize returns the hash's underlying block size.
//   - Inherited from the [hash.Hash] interface.
//
// [hash.Hash]: https://golang.org/pkg/hash/#Hash
func (t *TelegramMultipartHash) BlockSize() int {
	return sha512.BlockSize
}

// ? ----- Interface io.Writer -----

// Write writes len(p) bytes from p to the underlying data stream.
//   - Inherited from the [io.Writer] interface.
//
// [io.Writer]: https://golang.org/pkg/io/#Writer
func (t *TelegramMultipartHash) Write(p []byte) (n int, err error) {
	t.data = append(t.data, p...)
	return len(p), nil
}
