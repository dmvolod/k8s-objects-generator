package object_templates

import _ "embed"

//go:embed .gitignore.tmpl
var GitIgnore string

//go:embed object_kind.gotmpl
var ObjectKindTemplate string

//go:embed group_version.gotmpl
var GroupVersionTemplate string
