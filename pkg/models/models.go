package models

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

type CreateRouteRequest struct {
	Domain         string            `json:"domain"`
	Upstream       string            `json:"upstream"`
	TLS            *bool             `json:"tls"`
	Description    string            `json:"description"`
	Template       string            `json:"template"`
	TemplateParams map[string]string `json:"templateParams"`
	MatchConfig    string            `json:"matchConfig"`
	HandlerConfig  string            `json:"handlerConfig"`
}

type Template struct {
	Slug        string  `json:"slug"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Params      []Param `json:"params"`
}

type Param struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Placeholder string `json:"placeholder"`
	Required    bool   `json:"required"`
}

type ManagedTLD struct {
	TLD               string `json:"tld"`
	ResolverInstalled bool   `json:"resolverInstalled"`
	CreatedAt         string `json:"createdAt"`
}

type DaemonStatus struct {
	Status   string `json:"status"`
	Routes   int    `json:"routes"`
	TLDs     int    `json:"tlds"`
	Mkcert   bool   `json:"mkcert"`
	Platform string `json:"platform"`
}
