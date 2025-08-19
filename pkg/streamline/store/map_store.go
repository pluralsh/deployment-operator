package store

import (
	"fmt"
	"github.com/samber/lo"
	"sync"
)

type MapStore struct {
	mu      sync.RWMutex
	objects map[string]Entry
}

func NewWatchStore() Store {
	return &MapStore{
		objects: make(map[string]Entry),
	}
}

func (w *MapStore) Remove(id string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.objects, id)
	return nil
}

func (w *MapStore) List() ([]Entry, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return lo.Values(w.objects), nil
}

func (w *MapStore) Get(id string) (*Entry, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	obj, ok := w.objects[id]
	if !ok {
		return &obj, nil
	}
	return nil, fmt.Errorf("object with id %s doesn't exists", id)
}

func (w *MapStore) Save(entry Entry) error {
	if entry.UID == "" {
		return fmt.Errorf("entry ID can't be empty")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.objects[entry.UID] = entry
	return nil
}
