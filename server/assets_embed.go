package chserver

import (
	"embed"
)

// embeddedDashboardFS contains the dashboard static assets and templates.
// Paths are relative to this package directory (server/).
//
//go:embed dashboard/static dashboard/templates
var embeddedDashboardFS embed.FS
