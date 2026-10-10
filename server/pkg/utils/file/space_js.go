//go:build js && wasm

package file

import "errors"

func FreeSpace(_ string) (uint64, error) {
	return 0, errors.New("Workers has no disk storage; use streaming replication")
}
