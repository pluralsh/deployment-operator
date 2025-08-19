package store

type WatchStore struct {
}

func NewWatchStore() Store {
	return &WatchStore{}
}

func (w WatchStore) Remove(id string) error {

	return nil
}

func (w WatchStore) List() ([]Entry, error) {
	return nil, nil
}

func (w WatchStore) Get(id string) (*Entry, error) {
	return nil, nil
}

func (w WatchStore) Save(entry Entry) error {
	return nil
}
