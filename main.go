package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
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

var VERSION string

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

//go:embed logo.svg
var logoString string

func buildFooter(node *ConfigNode) *HTMLElement {
	footer := NewHTMLElement("footer")
	logoContainer := footer.AppendNew("div", Class("wyweb-logo"))
	logoContainer.AppendNew("span").AppendText("Powered by")
	logoContainer.AppendText(logoString)
	logoContainer.AppendNew("a", Href("https://wyweb.site"))
	copyrightMsg := footer.AppendNew("span", Class("copyright"))
	copyrightMsg.AppendText(
		fmt.Sprintf("Copyright © %d %s", time.Now().Year(), node.Copyright),
	)
	return footer
}

func buildDocument(bodyHTML *HTMLElement, headData HTMLHeadData, structuredData ...string) (bytes.Buffer, error) {
	var buf bytes.Buffer
	buf.WriteString("<!DOCTYPE html>\n")
	document := NewHTMLElement("html")
	head := buildHead(headData)
	for _, data := range structuredData {
		head.AppendNew("script", map[string]string{"type": "application+ld+json"}).AppendText(data)
	}
	document.Append(head)
	document.Append(bodyHTML)
	RenderHTML(document, &buf)
	return buf, nil
}

func breadcrumbs(node *ConfigNode, extraCrumbs ...WWNavLink) (*HTMLElement, string) {
	structuredData := map[string]interface{}{
		"@context": "https://schema.org",
		"@type":    "BreadcrumbList",
	}
	itemListElement := make([]map[string]interface{}, 0)
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
				Text: temp.Title,
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
	idx := 1
	for _, link := range crumbs {
		if link.Path == "" || link.Text == "" {
			continue
		}
		fullURL, _ := url.JoinPath("https://"+node.Tree.Domain, link.Path)
		itemListElement = append(itemListElement, map[string]interface{}{
			"@type":    "ListItem",
			"position": idx,
			"name":     link.Text,
			"item":     fullURL,
		})
		idx++
		crumb = ol.AppendNew("li").AppendNew("a", Href(link.Path))
		crumb.AppendText(link.Text)
	}
	structuredData["itemListElement"] = itemListElement
	crumb.Attributes["aria-current"] = "page"
	jsonld, _ := json.MarshalIndent(structuredData, "", "    ")
	return nav, string(jsonld)
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

func RouteTags(node *ConfigNode, taglist []string, w http.ResponseWriter, req *http.Request) {
	crumbs, bcsd := breadcrumbs(node, WWNavLink{Path: strings.TrimPrefix(req.URL.String(), "/"), Text: "Tags"})
	page := buildTagListing(node, taglist, crumbs)
	headData := node.Tree.GetDefaultHead()
	headData.Title = "Tags"
	buf, _ := buildDocument(page, *headData, bcsd)
	w.Write(buf.Bytes())
}

func RouteStatic(node *ConfigNode, w http.ResponseWriter) {
	var err error
	var structuredData []string
	meta := node.Data
	if node.HTML == nil {
		switch (*meta).(type) {
		//case *WyWebRoot:
		case *WyWebListing:
			structuredData, err = buildDirListing(node)
		case *WyWebPost:
			structuredData, err = buildPost(node)
		case *WyWebGallery:
			structuredData, err = buildGallery(node)
		default:
			w.WriteHeader(500)
			return
		}
		node.HTML.Append(buildFooter(node))
	}
	if err != nil {
		w.WriteHeader(404)
		w.Write([]byte(fileNotFound))
	}
	buf, _ := buildDocument(node.HTML, *node.GetHeadData(), structuredData...)
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
	path, _ := filepath.Rel(".", raw)
	if raw == "tags" {
		taglist := req.URL.Query()["tags"]
		RouteTags(realm.Root, taglist, w, req)
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

	if taglist, ok := req.URL.Query()["tags"]; ok {
		RouteTags(node, taglist, w, req)
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
	fmt.Printf("WyWeb version %s\n", VERSION)
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
	version := flag.Bool("v", false, "Print version and exit")
	flag.Parse()
	if *version {
		println(VERSION)
		os.Exit(0)
	}
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
