//go:generate gopherjs build

package main

import "github.com/gopherjs/gopherjs/js"

func main() {
	exports := js.Module.Get("exports")

	exports.Set("load", load)

	exports.Set("parsePost", post(parse))
	exports.Set("parseSignature", signature(parse))
	exports.Set("parseGeneric", raw(parse))
}

var app *js.Object

func load(data, callback *js.Object) {
	app = data.Get("app")

	callback.Invoke(nil)
}

func post(fn func(string) string) func(data, callback *js.Object) {
	return func(data, callback *js.Object) {
		if data != nil && data.Get("postData") != nil && data.Get("postData").Get("content") != nil {
			go func() {
				data.Get("postData").Set("content", fn(data.Get("postData").Get("content").String()))
				callback.Invoke(nil, data)
			}()
			return
		}
		callback.Invoke(nil, data)
	}
}

func signature(fn func(string) string) func(data, callback *js.Object) {
	return func(data, callback *js.Object) {
		if data != nil && data.Get("userData") != nil && data.Get("userData").Get("signature") != nil {
			go func() {
				data.Get("userData").Set("signature", fn(data.Get("userData").Get("signature").String()))
				callback.Invoke(nil, data)
			}()
			return
		}
		callback.Invoke(nil, data)
	}
}

func raw(fn func(string) string) func(raw string, callback *js.Object) {
	return func(raw string, callback *js.Object) {
		go func() {
			callback.Invoke(nil, fn(raw))
		}()
	}
}
