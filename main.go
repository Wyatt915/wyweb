package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"wyweb.site/wyweb/util"
)

const fileNotFound = `
<html>
<head><title>404 Not Found</title></head>
<body>
<center><h1>404 Not Found</h1></center>
</body>
</html>
`

func timer(name string) func() {
	start := time.Now()
	return func() {
		log.Printf("%s [%v]\n", name, time.Since(start))
	}
}

func buildHead(headData HTMLHeadData) *HTMLElement {
	head := NewHTMLElement("head")
	title := head.AppendNew("title")
	title.AppendText(headData.Title)
	for _, style := range headData.Styles {
		switch s := style.(type) {
		case URLResource:
			head.AppendNew("link", map[string]string{"rel": "stylesheet", "href": s.String}, s.Attributes)
		case RawResource:
			tag := head.AppendNew("style", s.Attributes)
			tag.AppendText(s.String)
		default:
			continue
		}
	}
	for _, script := range headData.Scripts {
		switch s := script.(type) {
		case URLResource:
			head.AppendNew("script", map[string]string{"src": s.String, "async": ""}, s.Attributes)
		case RawResource:
			tag := head.AppendNew("script", s.Attributes)
			tag.AppendText(s.String)
		default:
			continue
		}
	}
	head.AppendText(strings.Join(headData.Meta, "\n"))
	return head
}

func buildDocument(bodyHTML *HTMLElement, headData HTMLHeadData) (bytes.Buffer, error) {
	var buf bytes.Buffer
	buf.WriteString("<!DOCTYPE html>\n")
	document := NewHTMLElement("html")
	document.Append(buildHead(headData))
	document.Append(bodyHTML)
	RenderHTML(document, &buf)
	return buf, nil
}

func breadcrumbs(node *ConfigNode, extraCrumbs ...WWNavLink) *HTMLElement {
	nav := NewHTMLElement("nav", AriaLabel("Breadcrumbs"))
	ol := nav.AppendNew("ol", Class("breadcrumbs"))
	var crumbs []WWNavLink
	if node != nil {
		pathList := util.PathToList(node.Path)
		crumbs = make([]WWNavLink, 1+len(pathList))
		temp := node
		idx := len(pathList)
		for temp != nil && idx >= 0 {
			crumbs[idx] = WWNavLink{
				Path: "/" + temp.Path,
				Text: temp.Resolved.Title,
			}
			idx--
			temp = temp.Parent
		}
	} else {
		crumbs = make([]WWNavLink, 0)
	}
	if extraCrumbs != nil {
		crumbs = slices.Concat(crumbs, extraCrumbs)
	}
	var crumb *HTMLElement
	for _, link := range crumbs {
		crumb = ol.AppendNew("li").AppendNew("a", Href(link.Path))
		crumb.AppendText(link.Text)
	}
	crumb.Attributes["aria-current"] = "page"
	return nav
}

func GetRemoteAddr(req *http.Request) string {
	forwarded := req.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		return forwarded
	}
	return req.RemoteAddr
}

func GetHost(req *http.Request) string {
	forwarded := req.Header.Get("X-Forwarded-Host")
	if forwarded != "" {
		return forwarded
	}
	return req.Host
}

func RouteTags(tree *ConfigTree, w http.ResponseWriter, req *http.Request) {
	query, err := url.ParseQuery(req.URL.RawQuery)
	if err != nil {
		w.WriteHeader(404)
		w.Write([]byte(fileNotFound))
		return
	}
	referer, _ := url.Parse(req.Referer())
	var crumbs *HTMLElement
	if referer.Hostname() == GetHost(req) {
		refPath := strings.TrimPrefix(referer.Path, "/")
		refNode, err := tree.Search(refPath)
		if err == nil {
			crumbs = breadcrumbs(refNode, WWNavLink{Path: req.URL.String(), Text: "Tags"})
		}
	}
	page := buildTagListing(tree, query, crumbs)
	headData := tree.GetDefaultHead()
	headData.Title = "Tags"
	buf, _ := buildDocument(page, *headData)
	w.Write(buf.Bytes())
}

func RouteStatic(node *ConfigNode, w http.ResponseWriter) {
	var err error
	meta := node.Data
	if node.Resolved.HTML == nil {
		switch (*meta).(type) {
		//case *WyWebRoot:
		case *WyWebListing:
			err = buildDirListing(node)
		case *WyWebPost:
			err = buildPost(node)
		case *WyWebGallery:
			err = buildGallery(node)
		default:
			w.WriteHeader(500)
			return
		}
	}
	if err != nil {
		w.WriteHeader(404)
		w.Write([]byte(fileNotFound))
	}
	buf, _ := buildDocument(node.Resolved.HTML, *node.GetHeadData())
	w.Write(buf.Bytes())
}

type WorldTree struct {
	sync.RWMutex
	realms map[string]*ConfigTree
}

// Get a branch or create it if it does not exist
func (wt *WorldTree) GetRealm(host string) (*ConfigTree, error) {
	wt.RLock()
	realm, ok := wt.realms[host]
	wt.RUnlock()
	if !ok {
		wt.Lock()
		defer wt.Unlock()
		var err error
		realm, err = BuildConfigTree(".", host)
		if err != nil {
			return nil, err
		}
		wt.realms[host] = realm
	}
	return realm, nil
}

func (wt *WorldTree) Len() int {
	wt.Lock()
	defer wt.Unlock()
	return len(wt.realms)
}

var GlobalTree WorldTree

type WyWebHandler struct {
	http.Handler
	Yggdrasil *WorldTree
}

func (r WyWebHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	defer timer(fmt.Sprintf("%s: %s requested %s", GetHost(req), GetRemoteAddr(req), req.RequestURI))()
	docRoot := req.Header["Document-Root"][0]
	os.Chdir(docRoot)
	realm, err := r.Yggdrasil.GetRealm(GetHost(req))
	if err != nil {
		w.WriteHeader(500)
		return
	}
	raw := strings.TrimPrefix(req.URL.Path, "/")
	path, _ := filepath.Rel(".", raw) // remove that pesky leading slash
	if path == "tags" {
		RouteTags(realm, w, req)
		return
	}
	node, err := realm.Search(path)
	if err != nil {
		_, ok := os.Stat(filepath.Join(path, "wyweb"))
		if ok != nil {
			w.WriteHeader(404)
			w.Write([]byte(fileNotFound))
			return
		}
		w.WriteHeader(404)
		w.Write([]byte(fileNotFound))
		log.Printf("Bizarro error bruv\n")
		return
	}

	RouteStatic(node, w)
}

func TryListen(sockfile string) (net.Listener, error) {
	var socket net.Listener
	for {
		var err error
		socket, err = net.Listen("unix", sockfile)
		if err != nil {
			lsof := exec.Command("lsof", "+E", "-t", sockfile)
			var out bytes.Buffer
			lsof.Stdout = &out
			lsof.Run()
			if out.Len() == 0 {
				os.Remove(sockfile)
			} else {
				return socket, fmt.Errorf("%s in use by %s", sockfile, out.String())
			}
		} else {
			break
		}
	}
	return socket, nil
}

func TryChown(sockfile, group string) error {
	grp, err := user.LookupGroup(group)
	if err != nil {
		return fmt.Errorf("could not find specified group '%s'", group)
	}
	gid, _ := strconv.Atoi(grp.Gid)
	if err = os.Chown(sockfile, -1, gid); err != nil {
		return fmt.Errorf("failed to change ownership: %v", err)
	}
	err = os.Chmod(sockfile, 0660)
	if err != nil {
		return fmt.Errorf("could not change permissions for %s", sockfile)
	}
	return nil
}

func WyWebStart(sockfile, group string) {
	defer os.Remove(sockfile)
	socket, err := TryListen(sockfile)
	if err != nil {
		log.Println(err.Error())
		return
	}
	err = TryChown(sockfile, group)
	if err != nil {
		log.Printf("WARN: %s", err.Error())
	}
	GlobalTree.realms = make(map[string]*ConfigTree)
	handler := WyWebHandler{
		Yggdrasil: &GlobalTree,
	}
	//	handler.tree = new(ConfigTree)
	http.Serve(socket, handler)
}

func main() {
	sock := flag.String("sock", "/tmp/wyweb.sock", "Path to the unix domain socket used by WyWeb")
	grp := flag.String("grp", "www-data", "Group of the unix domain socket used by WyWeb (Should be the accessible by your reverse proxy)")
	flag.Parse()
	log.SetFlags(log.Lshortfile)
	// Cleanup the sockfile.
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		os.Remove(*sock)
		os.Exit(1)
	}()
	WyWebStart(*sock, *grp)
}
