package sync

/*type Resource struct {
	ServiceId string
	Sha       string
}

func Sha(id string, key kube.ResourceKey) string {
	h := sha256.New()
	_, _ = h.Write([]byte(strings.Join([]string{id, key.Group, key.Kind, key.Name, key.Namespace}, ".")))
	return "sha256." + base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

func NewResource(id, sha string) *Resource {
	return &Resource{ServiceId: id, Sha: sha}
}*/
