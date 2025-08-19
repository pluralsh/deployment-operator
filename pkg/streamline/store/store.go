package store

type Entry struct {
	UID       string
	ParentUID string
	Group     string
	Version   string
	Kind      string
	Name      string
	Namespace string
	Status    string
	ServiceID string
}

type Store interface {
	Remove(id string) error
	List() ([]Entry, error)
	Get(id string) (*Entry, error)
	Save(entry Entry) error
}
