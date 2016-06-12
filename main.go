//go:generate gopherjs build

package main

import "github.com/gopherjs/gopherjs/js"

func main() {
	exports := js.Module.Get("exports")

	exports.Set("parsePost", post(parse))
	exports.Set("parseSignature", signature(parse))
	exports.Set("parseGeneric", raw(parse))
}

func post(fn func(string) string) func(data, callback *js.Object) {
	return func(data, callback *js.Object) {
		go func() {
			if data != nil && data.Get("postData") != nil && data.Get("postData").Get("content") != nil {
				data.Get("postData").Set("content", fn(data.Get("postData").Get("content").String()))
				callback.Invoke(nil, data)
			}
		}()
	}
}

func signature(fn func(string) string) func(data, callback *js.Object) {
	return func(data, callback *js.Object) {
		go func() {
			if data != nil && data.Get("userData") != nil && data.Get("userData").Get("signature") != nil {
				data.Get("userData").Set("signature", fn(data.Get("userData").Get("signature").String()))
				callback.Invoke(nil, data)
			}
		}()
	}
}

func raw(fn func(string) string) func(raw string, callback *js.Object) {
	return func(raw string, callback *js.Object) {
		go func() {
			callback.Invoke(nil, fn(raw))
		}()
	}
}
