package test

import (
	"syscall/js"
	"testing"
	"time"
)

func TestWebGL(t *testing.T) {
	window := js.Global().Get("window")
	document := window.Get("document")
	body := document.Get("body")
	canvas := document.Call("createElement", "canvas")
	body.Call("appendChild", canvas)
	if context := canvas.Call("getContext", "2d"); !context.Truthy() {
		t.Error("getContext('2d') failed")
	}
	if context := canvas.Call("getContext", "webgl"); !context.Truthy() {
		t.Error("getContext('webgl') failed")
	}

	time.Sleep(30 * time.Second)
}
