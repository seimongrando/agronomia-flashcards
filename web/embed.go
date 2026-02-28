package web

import "embed"

//go:embed all:static *.html
var Content embed.FS
