package stable

import (
	"fmt"
	"os"
	"time"
)

// ExclusiveOpener tries to open a file exclusively. Injected for tests / Windows.
type ExclusiveOpener func(path string) error

type Options struct {
	Wait     time.Duration
	Poll     time.Duration
	Timeout  time.Duration
	Exclusive ExclusiveOpener
}

func WaitReady(path string, opt Options) error {
	if opt.Wait <= 0 {
		opt.Wait = 3 * time.Second
	}
	if opt.Poll <= 0 {
		opt.Poll = 500 * time.Millisecond
	}
	if opt.Timeout <= 0 {
		opt.Timeout = 2 * time.Minute
	}
	deadline := time.Now().Add(opt.Timeout)
	var lastSize int64 = -1
	var lastMod time.Time
	stableSince := time.Time{}

	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("stable wait timeout")
		}
		fi, err := os.Stat(path)
		if err != nil {
			return err
		}
		if fi.IsDir() {
			return fmt.Errorf("is directory")
		}
		size := fi.Size()
		mod := fi.ModTime()
		if size == lastSize && mod.Equal(lastMod) {
			if stableSince.IsZero() {
				stableSince = time.Now()
			}
			if time.Since(stableSince) >= opt.Wait {
				if opt.Exclusive != nil {
					if err := opt.Exclusive(path); err != nil {
						stableSince = time.Time{}
						time.Sleep(opt.Poll)
						continue
					}
				}
				return nil
			}
		} else {
			lastSize = size
			lastMod = mod
			stableSince = time.Time{}
		}
		time.Sleep(opt.Poll)
	}
}
