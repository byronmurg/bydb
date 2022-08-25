package document

type Document struct {
	Id string `json:"id"`
	Part string `json:"part"`
	Index map[string]any `json:"index"`
	Categories []string `json:"categories"`
	Block map[string]any `json:"block"`
	Updated int64 `updated`
	Created int64 `created`
}
