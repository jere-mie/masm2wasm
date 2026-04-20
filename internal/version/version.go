package version

import "fmt"

var (
	Version   = "0.1.2"
	Commit    = "dev"
	BuildDate = ""
)

func Summary() string {
	if BuildDate == "" {
		return fmt.Sprintf("masm2wasm %s", Version)
	}
	return fmt.Sprintf("masm2wasm %s (%s, %s)", Version, Commit, BuildDate)
}
