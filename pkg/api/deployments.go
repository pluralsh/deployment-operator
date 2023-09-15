package api

type Deployment struct {
	Id        string
	Git       Git
	Subfolder string
	Namespace string
}

type Git struct {
	Url string
	Ref string
}
