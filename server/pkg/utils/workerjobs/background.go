//go:build !js

package workerjobs

func Background(task func()) { go task() }
