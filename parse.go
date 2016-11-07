package main

import (
	"bytes"
	"fmt"
	"image"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/BenLubar/nodejs-roundtripper"
	"github.com/gopherjs/gopherjs/js"
	"github.com/karlseguin/ccache"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const debug = false

var nconf = js.Module.Get("parent").Call("require", "nconf")
var winston = js.Module.Get("parent").Call("require", "winston")
var lru = ccache.New(ccache.Configure())
var client = &http.Client{
	Transport: roundtripper.RoundTripper,
	Timeout:   time.Second * 15,
}

func parse(src string) string {
	nodes, err := html.ParseFragment(strings.NewReader(src), &html.Node{
		Type:     html.ElementNode,
		Data:     "div",
		DataAtom: atom.Div,
	})
	if err != nil {
		return src
	}

	var wg sync.WaitGroup
	wg.Add(len(nodes))
	for _, n := range nodes {
		go parseNode(&wg, n)
	}
	wg.Wait()

	var buf bytes.Buffer
	for _, n := range nodes {
		err = html.Render(&buf, n)
		if err != nil {
			return src
		}
	}
	return buf.String()
}

func parseNode(wg *sync.WaitGroup, n *html.Node) {
	defer wg.Done()

	for {
		if n.Type == html.ElementNode && n.DataAtom == atom.Img {
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

		if n.FirstChild != nil {
			n = n.FirstChild
		} else {
			p := n
			for p != nil && p.NextSibling == nil {
				p = p.Parent
			}
			if p == nil {
				break
			}

			n = p.NextSibling
		}
	}
}

func setSize(wg *sync.WaitGroup, n *html.Node, src string) {
	defer wg.Done()

	u, err := url.Parse(nconf.Call("get", "url").String())
	if err != nil {
		return
	}
	originalHost := u.Host
	originalPath := u.Path
	u, err = u.Parse(src)
	if err != nil {
		return
	}
	cleanPath := path.Clean(u.Path)
	if u.Scheme != "http" && u.Scheme != "https" {
		return
	}
	if strings.HasSuffix(u.Path, ".php") || strings.HasSuffix(u.Path, ".svg") {
		return
	}
	src = u.String()

	item, err := lru.Fetch(src, time.Minute*10, func() (interface{}, error) {
		if u.Host == originalHost {
			if uploadURL := nconf.Call("get", "upload_url").String(); strings.HasPrefix(cleanPath, uploadURL) {
				localPath := filepath.Join(nconf.Call("get", "base_dir").String(), nconf.Call("get", "upload_path").String(), strings.TrimPrefix(cleanPath, uploadURL))
				f, err := os.Open(localPath)
				if err != nil {
					if debug {
						winston.Call("warn", fmt.Sprintf("[nodebb-plugin-image-size] os.Open %q %v", localPath, err))
					}
					return image.Config{}, nil
				}
				defer f.Close()

				config, _, err := image.DecodeConfig(f)
				if err != nil {
					if debug {
						winston.Call("warn", fmt.Sprintf("[nodebb-plugin-image-size] image.DecodeConfig %q %v", localPath, err))
					}
					return image.Config{}, nil
				}
				return config, nil
			}
			if strings.HasPrefix(cleanPath, originalPath) {
				return image.Config{}, nil
			}
		}

		req, err := http.NewRequest("GET", src, nil)
		if err != nil {
			if debug {
				winston.Call("warn", fmt.Sprintf("[nodebb-plugin-image-size] http.NewRequest %q %v", src, err))
			}
			return nil, err
		}
		req.Header.Set("Accept", "image/*")
		req.Header.Set("User-Agent", "nodebb-plugin-image-size/0.0 (+https://github.com/BenLubar/nodebb-plugin-image-size)")
		resp, err := client.Do(req)
		if err != nil {
			if debug {
				winston.Call("warn", fmt.Sprintf("[nodebb-plugin-image-size] client.Do %q %v", src, err))
			}
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			if debug {
				winston.Call("warn", fmt.Sprintf("[nodebb-plugin-image-size] response status %q %s", src, resp.Status))
			}
			return image.Config{}, nil
		}

		config, _, err := image.DecodeConfig(resp.Body)
		if err != nil {
			if debug {
				winston.Call("warn", fmt.Sprintf("[nodebb-plugin-image-size] image.DecodeConfig %q %v", src, err))
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
