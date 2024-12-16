package autoregistration

type extensionConfigAO struct {
	UnixSocket string   `json:"unixSocket,omitempty"`
	Url        string   `json:"url,omitempty"`
	Types      []string `json:"types,omitempty"`
}
