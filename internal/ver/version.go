package ver

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"time"
)

func Load() Version {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return Version{
			Version:   "devel",
			GoVersion: runtime.Version(),
			Revision:  "unknown",
		}
	}

	var (
		revision  = "unknown"
		buildTime = "unknown"
		dirty     bool
	)
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "build.timestamp":
			buildTime = setting.Value
		case "vcs.dirty":
			dirty = setting.Value == "true"
		}
	}

	return Version{
		Version:   info.Main.Version,
		GoVersion: info.GoVersion,
		Revision:  revision,
		BuildTime: buildTime,
		Dirty:     dirty,
	}
}

type Version struct {
	Version   string
	GoVersion string
	Revision  string
	BuildTime string
	Dirty     bool
}

func (v Version) Format() string {
	commit := v.Revision
	if len(commit) > 7 {
		commit = commit[:7]
	}

	var buildTimeStr string
	buildTime, err := time.Parse(time.RFC3339, v.BuildTime)
	if err != nil {
		buildTimeStr = "unknown"
	} else {
		buildTimeStr = buildTime.Format(time.ANSIC)
	}

	return fmt.Sprintf("Go Version: %s\nVersion: %s\nCommit: %s\nBuild Time: %s\nOS/Arch: %s/%s\n", v.GoVersion, v.Version, commit, buildTimeStr, runtime.GOOS, runtime.GOARCH)
}
