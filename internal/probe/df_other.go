//go:build !linux && !darwin

package probe

import "errors"

func setDontFragment(int) error { return errors.New("not supported on this OS") }
