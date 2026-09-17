// Package web embeds the self-contained bilingual demo UI.
package web

import _ "embed"

//go:embed index.html
var indexHTML string

// IndexHTML returns the embedded single-file UI document.
func IndexHTML() string { return indexHTML }
