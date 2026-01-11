package auth

import (
	_ "embed"
	"html/template"
	"io"
)

//go:embed templates/waiting.html
var waitingTemplate string

//go:embed templates/success.html
var successTemplate string

//go:embed templates/error.html
var errorTemplate string

// SuccessTemplateData holds data for the success page
type SuccessTemplateData struct {
	Profile          string
	CountdownSeconds int
}

// ErrorTemplateData holds data for the error page
type ErrorTemplateData struct {
	Error string
}

// renderWaitingPage renders the waiting/authenticating page
func renderWaitingPage(w io.Writer) {
	tmpl, err := template.New("waiting").Parse(waitingTemplate)
	if err != nil {
		_, _ = w.Write([]byte("Authenticating with roam... Please complete the login in your browser."))
		return
	}
	_ = tmpl.Execute(w, nil)
}

// renderSuccessPage renders the authentication success page
func renderSuccessPage(w io.Writer) {
	tmpl, err := template.New("success").Parse(successTemplate)
	if err != nil {
		_, _ = w.Write([]byte("Authentication successful! You can close this window and return to the terminal."))
		return
	}
	data := SuccessTemplateData{
		CountdownSeconds: PostSuccessDisplaySeconds,
	}
	_ = tmpl.Execute(w, data)
}

// renderErrorPage renders the authentication error page
func renderErrorPage(w io.Writer, errorMsg string) {
	tmpl, err := template.New("error").Parse(errorTemplate)
	if err != nil {
		_, _ = w.Write([]byte("Authentication failed: " + errorMsg))
		return
	}
	_ = tmpl.Execute(w, ErrorTemplateData{Error: errorMsg})
}
