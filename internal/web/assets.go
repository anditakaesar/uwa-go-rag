package web

import "embed"

//go:embed templates/*.html
var TemplatesFS embed.FS

//go:embed public/*
var PublicFS embed.FS
