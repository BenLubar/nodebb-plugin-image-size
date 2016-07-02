package main

import (
	"bytes"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/BenLubar/nodejs-roundtripper"
	"github.com/gopherjs/gopherjs/js"
	"github.com/karlseguin/ccache"
	_ "github.com/mat/besticon/ico"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"
	"golang.org/x/net/html"
)

const debug = false

var nconf = js.Module.Get("parent").Call("require", "nconf")
var lru = ccache.New(ccache.Configure())
var client = &http.Client{
	Transport: roundtripper.RoundTripper,
	Timeout:   time.Second * 5,
}

func parse(src string) string {
	doc, err := html.Parse(strings.NewReader(src))
	if err != nil {
		return src
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go parseNode(&wg, doc)
	wg.Wait()

	var buf bytes.Buffer
	err = html.Render(&buf, doc)
	if err != nil {
		return src
	}
	return buf.String()
}

func parseNode(wg *sync.WaitGroup, n *html.Node) {
	defer wg.Done()

	if n.Type == html.ElementNode && n.Data == "img" {
		var src, width, height string
		for _, a := range n.Attr {
			switch a.Key {
			case "src":
				src = a.Val
			case "width":
				width = a.Val
			case "height":
				height = a.Val
			}
		}

		_, err := strconv.Atoi(width)
		if err == nil {
			_, err = strconv.Atoi(height)
		}
		if err != nil {
			wg.Add(1)
			go setSize(wg, n, src)
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		wg.Add(1)
		parseNode(wg, c)
	}
}

func setSize(wg *sync.WaitGroup, n *html.Node, src string) {
	defer wg.Done()

	u, err := url.Parse(nconf.Call("get", "url").String())
	if err != nil {
		return
	}
	originalHost := u.Host
	u, err = u.Parse(src)
	if err != nil {
		return
	}
	if u.Host == originalHost && strings.HasSuffix(u.Path, ".svg") {
		return
	}
	src = u.String()

	item, err := lru.Fetch(src, time.Minute*10, func() (interface{}, error) {
		req, err := http.NewRequest("GET", src, nil)
		if err != nil {
			if debug {
				log.Println("[nodebb-plugin-image-size]", err)
			}
			return nil, err
		}
		req.Header.Set("Accept", "image/*, */*;q=0.1")
		req.Header.Set("User-Agent", "nodebb-plugin-image-size/0.0 (+https://github.com/BenLubar/nodebb-plugin-image-size)")
		resp, err := client.Do(req)
		if err != nil {
			if debug {
				log.Println("[nodebb-plugin-image-size]", err)
			}
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			if debug {
				log.Println("[nodebb-plugin-image-size]", src, resp.Status)
			}
			return image.Config{}, nil
		}

		config, _, err := image.DecodeConfig(resp.Body)
		if err != nil {
			if debug {
				log.Println("[nodebb-plugin-image-size]", src, err)
			}
			return image.Config{}, nil
		}
		return config, nil
	})
	if err != nil {
		// nothing we can do
		return
	}

	config := item.Value().(image.Config)
	if config.Width == 0 || config.Height == 0 {
		return
	}

	for i, a := range n.Attr {
		switch a.Key {
		case "width":
			n.Attr[i].Val = strconv.Itoa(config.Width)
			config.Width = 0
		case "height":
			n.Attr[i].Val = strconv.Itoa(config.Height)
			config.Height = 0
		}
	}

	if config.Width != 0 {
		n.Attr = append(n.Attr, html.Attribute{
			Key: "width",
			Val: strconv.Itoa(config.Width),
		})
	}
	if config.Height != 0 {
		n.Attr = append(n.Attr, html.Attribute{
			Key: "height",
			Val: strconv.Itoa(config.Height),
		})
	}
}
