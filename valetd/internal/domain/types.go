package domain

type AddRouteRequest struct {
	Domain         string            `json:"domain"`
	Upstream       string            `json:"upstream"`
	Description    string            `json:"description"`
	Template       string            `json:"template"`
	TemplateParams map[string]string `json:"templateParams"`
	MatchConfig    string            `json:"matchConfig"`
	HandlerConfig  string            `json:"handlerConfig"`
	TLSUpstream    bool              `json:"tlsUpstream"`
}

type UpdateRouteRequest struct {
	Domain        *string `json:"domain"`
	Upstream      *string `json:"upstream"`
	Description   *string `json:"description"`
	MatchConfig   *string `json:"matchConfig"`
	HandlerConfig *string `json:"handlerConfig"`
	Template      *string `json:"template"`
	TLSUpstream   *bool   `json:"tlsUpstream"`
}

type DiagnosticCheck struct {
	Check   string `json:"check"`
	Status  string `json:"status"` // "pass" or "fail"
	Details string `json:"details,omitempty"`
}

type DiagnosticResult struct {
	Domain string            `json:"domain"`
	Checks []DiagnosticCheck `json:"checks"`
}

type TLDStatus struct {
	TLD               string `json:"tld"`
	ResolverInstalled bool   `json:"resolverInstalled"`
	CreatedAt         string `json:"createdAt"`
}
