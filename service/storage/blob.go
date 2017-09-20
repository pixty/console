package storage

import (
	"io"
	"time"
)

type (
	BlobMeta struct {
		Id        string
		Timestamp time.Time
		Size      int64
	}

	BlobStorage interface {
		// Saves BLOB to the store. The invoker should close the reader itself
		Add(r io.Reader, bMeta *BlobMeta) (string, error)

		// Returns reader for the object ID. It is invoker responsibility to
		// close the reader after use.
		Read(objId string) (io.ReadCloser, *BlobMeta)

		// Reads meta data for the object. Returns nil if not found.
		ReadMeta(objId string) *BlobMeta

		// Deletes an object by its id. Returns error != nil if operation is failed
		Delete(objId ...string) error

		// Deletes all ids with prefix. Returns number of objects deleted
		DeleteAllWithPrefix(prefix string) int
	}
)
