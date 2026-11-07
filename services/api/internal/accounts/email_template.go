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
	message, err := renderVerificationEmail(code, "sign_in")
	return message.HTML, message.Text, err
}

type verificationEmail struct {
	Subject, HTML, Text string
}

func renderVerificationEmail(code, purpose string) (verificationEmail, error) {
	data := struct {
		Code, Subject, Preheader, Heading, Instruction, Footer string
	}{
		Code:        code,
		Subject:     "Your Syntelle administration code",
		Preheader:   "Your one-time administration code is valid for 10 minutes.",
		Heading:     "Your administration code",
		Instruction: "Enter this code in the Syntelle administration window where you requested it.",
		Footer:      "Sent by Syntelle to verify an administrative action.",
	}
	textLabel := "Your Syntelle administration code is: "
	switch purpose {
	case "sign_in":
		data.Subject = "Your Syntelle sign-in code"
		data.Preheader = "Your one-time sign-in code is valid for 10 minutes."
		data.Heading = "Your sign-in code"
		data.Instruction = "Enter this code in the Syntelle window where you requested it."
		data.Footer = "Sent by Syntelle to help you sign in. No password required."
		textLabel = "Your Syntelle sign-in code is: "
	case "ownership_transfer":
		data.Subject = "Your Syntelle ownership code"
		data.Preheader = "Your one-time ownership verification code is valid for 10 minutes."
		data.Heading = "Your ownership verification code"
		data.Instruction = "Enter this code in the Syntelle ownership transfer window where you requested it."
		data.Footer = "Sent by Syntelle to verify an ownership transfer."
		textLabel = "Your Syntelle ownership verification code is: "
	}
	var html bytes.Buffer
	if err := signInTemplate.Execute(&html, data); err != nil {
		return verificationEmail{}, err
	}
	text := textLabel + code + "\n\nIt expires in 10 minutes and can only be used once. If you did not request this, ignore this email."
	return verificationEmail{Subject: data.Subject, HTML: html.String(), Text: text}, nil
}
