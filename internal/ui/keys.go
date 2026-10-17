package ui

import (
	"io"
	"sync"
	"time"
)

// Key sequences for Keys.
const (
	Enter = "\r"
	Down  = "\x1b[B"
	Up    = "\x1b[A"
	Space = " "
	Tab   = "\t"
)

// Keys returns a reader that delivers one key per read, a little apart, the
// way a terminal does. Forms read a whole chunk as pasted text, so tests
// that drive a form (NewTerminal) type into it with Keys. Strings are split
// into single characters; Enter, Down and the like are sent whole.
func Keys(keys ...string) io.Reader {
	var split []string
	for _, k := range keys {
		if len(k) > 1 && k[0] != '\x1b' {
			for _, r := range k {
				split = append(split, string(r))
			}
			continue
		}
		split = append(split, k)
	}
	return &keyReader{keys: split}
}

type keyReader struct {
	mu   sync.Mutex
	keys []string
}

func (k *keyReader) Read(p []byte) (int, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if len(k.keys) == 0 {
		return 0, io.EOF
	}
	time.Sleep(15 * time.Millisecond)
	n := copy(p, k.keys[0])
	k.keys = k.keys[1:]
	return n, nil
}
