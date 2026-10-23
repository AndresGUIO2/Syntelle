package accounts

import (
	"bytes"
	_ "embed"
	"html/template"
)

//go:embed templates/sign-in.html
var signInHTML string

var signInTemplate = template.Must(template.New("sign-in").Parse(signInHTML))

// Render only in memory. Actual codes and rendered login emails must never
// enter logs, diagnostics, database evidence or browser resources.
func renderSignInEmail(code string) (string, string, error) {
	var html bytes.Buffer
	if err := signInTemplate.Execute(&html, struct{ Code string }{Code: code}); err != nil {
		return "", "", err
	}
	text := "Your Syntelle sign-in code is: " + code + "\n\nIt expires in 10 minutes and can only be used once. If you did not request this, ignore this email."
	return html.String(), text, nil
}
