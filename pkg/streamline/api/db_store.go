package api

// Storage defines the storage options for the database.
type Storage string

const (
	// StorageMemory stores data in-memory.
	StorageMemory Storage = ":memory:"

	// StorageFile stores data in a file on a disk.
	StorageFile Storage = "file"
)
