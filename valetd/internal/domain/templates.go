package domain

import (
	"fmt"

	"github.com/loaapp/valet/valetd/internal/templates"
)

func ResolveTemplate(req *AddRouteRequest) error {
	if req.Template == "" {
		return nil
	}
	tmpl := templates.Get(req.Template)
	if tmpl == nil {
		return fmt.Errorf("unknown template: %s", req.Template)
	}

	params := make(map[string]string)
	if req.Domain != "" {
		params["domain"] = req.Domain
	}
	if req.Upstream != "" {
		params["upstream"] = req.Upstream
	}
	for k, v := range req.TemplateParams {
		params[k] = v
	}

	// Validate required params
	for _, p := range tmpl.Params {
		if p.Required && params[p.Key] == "" {
			return fmt.Errorf("template %s requires parameter %q", req.Template, p.Key)
		}
	}

	matchConfig, handlerConfig, err := tmpl.Apply(params)
	if err != nil {
		return fmt.Errorf("template %s: %w", req.Template, err)
	}
	if matchConfig != "" {
		req.MatchConfig = matchConfig
	}
	if handlerConfig != "" {
		req.HandlerConfig = handlerConfig
	}
	return nil
}
