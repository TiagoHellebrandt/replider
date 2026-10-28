// Package oauthpage holds the server-rendered login+consent page for
// the OAuth 2.1 authorization flow (remote MCP connectors). Rendered by
// the API itself — the Flutter dashboard is a separate origin and the
// authorize leg must not depend on it.
package oauthpage

import (
	_ "embed"
	"html/template"
	"io"
)

//go:embed authorize.html
var authorizeHTML string

var authorizeTmpl = template.Must(template.New("authorize").Parse(authorizeHTML))

// View is the template payload. Mode selects the visible form section:
// "login", "mfa", or "error".
type View struct {
	Mode        string
	ClientName  string
	Error       string
	Email       string
	FlowToken   string
	MfaSession  string
	ClientID    string
	RedirectURI string
	State       string
	Scope       string
	Challenge   string
	Method      string
}

func Render(w io.Writer, v View) error {
	return authorizeTmpl.Execute(w, v)
}
