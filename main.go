package main

import (
	"github.com/containerinfra/kube-pg-upgrade/cmd/kube-pg-upgrade/app"
	versionpkg "github.com/containerinfra/kube-pg-upgrade/pkg/version"
)

// these must be set by the compiler using LDFLAGS
// -X main.version= -X main.commit= -X main.date= -X main.builtBy=
var (
	version string
	commit  string
	date    string
	builtBy string
)

func main() {
	app.Execute()
}

func init() {
	versionpkg.Init(version, commit, date, builtBy)
}
