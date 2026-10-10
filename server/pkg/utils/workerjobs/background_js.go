//go:build js && wasm

package workerjobs

import (
	"fmt"
	"syscall/js"

	log "github.com/sirupsen/logrus"
	"github.com/syumai/workers-go/exp/cloudflare/durableobjects"
)

var BackgroundState *durableobjects.DurableObjectState

// Background keeps the event alive and releases the executor in this persistent runtime.
func Background(task func()) {
	executor := js.FuncOf(func(_ js.Value, args []js.Value) any {
		resolve := args[0]
		go func() {
			defer func() {
				if failure := recover(); failure != nil {
					log.Error(fmt.Errorf("Worker background task: %v", failure))
				}
				resolve.Invoke(js.Undefined())
			}()
			task()
		}()
		return nil
	})
	promise := js.Global().Get("Promise").New(executor)
	executor.Release()
	if err := BackgroundState.WaitUntil(promise); err != nil {
		log.WithError(err).Error("Worker waitUntil failed")
	}
}
