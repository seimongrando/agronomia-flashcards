package web

import "embed"

//go:embed all:static index.html study.html me.html teach.html deck_manage.html admin_users.html
var Content embed.FS
