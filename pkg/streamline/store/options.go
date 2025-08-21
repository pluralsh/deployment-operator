package store

// Option represents a function that configures the database store.
type Option func(store *DatabaseStore)

// WithPoolSize sets the maximum number of concurrent connections in the pool.
func WithPoolSize(size int) Option {
	return func(in *DatabaseStore) {
		in.poolSize = size
	}
}

// WithStorage sets the storage mode for the cache.
func WithStorage(storage Storage) Option {
	return func(in *DatabaseStore) {
		in.storage = storage
	}
}

// WithFilePath sets the path where the cache file will be stored in the file storage mode.
func WithFilePath(path string) Option {
	return func(in *DatabaseStore) {
		in.filePath = path
	}
}
