//go:build !darwin && !linux

package verify

import (
	"errors"
	"os"
)

func lockFile(*os.File) error {
	return errors.New("verification evidence locks are supported only on macOS and Linux")
}
func unlockFile(*os.File) error { return nil }
