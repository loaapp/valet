package db

type Route struct {
	ID            string `json:"id"`
	Domain        string `json:"domain"`
	Upstream      string `json:"upstream"`
	TLSEnabled    bool   `json:"tlsEnabled"`
	CertPath      string `json:"certPath"`
	KeyPath       string `json:"keyPath"`
	MatchConfig   string `json:"matchConfig"`
	HandlerConfig string `json:"handlerConfig"`
	Template      string `json:"template"`
	Description   string `json:"description"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
}

type ManagedTLD struct {
	TLD               string `json:"tld"`
	ResolverInstalled bool   `json:"resolverInstalled"`
	CreatedAt         string `json:"createdAt"`
}

type Setting struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type DNSEntry struct {
	Domain    string `json:"domain"`
	TLD       string `json:"tld"`
	Target    string `json:"target"`
	CreatedAt string `json:"createdAt"`
}
