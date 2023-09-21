package sync

type Resource struct {
	ServiceId string
}

func NewResource(id string) *Resource {
	return &Resource{ServiceId: id}
}
