//go:build !darwin && !linux

package claim

import (
	"errors"
	"os"
)

func lockFile(*os.File) error {
	return errors.New("claim advisory locks are supported only on macOS and Linux")
}
func unlockFile(*os.File) error { return nil }
