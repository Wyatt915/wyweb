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
	"syscall"
	"time"

	"go.abhg.dev/goldmark/toc"

	"wyweb.site/wyweb/util"
)

var globalTree *ConfigTree

const fileNotFound = `
<html>
<head><title>404 Not Found</title></head>
<body>
<center><h1>404 Not Found</h1></center>
</body>
</html>
`

func check(e error) {
	if e != nil {
		fmt.Println(e.Error())
		panic(e)
	}
}
func timer(name string) func() {
	start := time.Now()
	return func() {
		log.Printf("%s [%v]\n", name, time.Since(start))
	}
}

func tocRecurse(table *toc.Item, parent *HTMLElement) {
	for _, item := range table.Items {
		child := parent.AppendNew("li")
		child.AppendNew("a", Href("#"+string(item.ID))).AppendText(string(item.Title))
		if len(item.Items) > 0 {
			ul := child.AppendNew("ul")
			tocRecurse(item, ul)
		}
	}
}

func renderTOC(t *toc.TOC) *HTMLElement {
	if len(t.Items) == 0 {
		return nil
	}
	elem := NewHTMLElement("nav", Class("nav-toc"))
	ul := elem.AppendNew("div", Class("toc")).AppendNew("ul")
	for _, item := range t.Items {
		tocRecurse(item, ul)
	}
	if len(ul.Children) == 0 {
		return nil
	}
	return elem
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

func findIndex(path string) ([]byte, error) {
	tryFiles := []string{
		"article.md",
		"index.md",
		"post.md",
		"article",
		"index",
		"post",
	}
	for _, f := range tryFiles {
		index := filepath.Join(path, f)
		_, err := os.Stat(index)
		if err == nil {
			return os.ReadFile(index)
		}
	}
	return nil, fmt.Errorf("could not find index")
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

type WyWebHandler struct {
	http.Handler
}

func (r WyWebHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	defer timer(fmt.Sprintf("%s requested by %s", req.RequestURI, GetRemoteAddr(req)))()
	log.Println(req.URL.Hostname(), GetHost(req))
	docRoot := req.Header["Document-Root"][0]
	os.Chdir(docRoot)
	if globalTree == nil {
		var e error
		globalTree, e = BuildConfigTree(".", GetHost(req))
		check(e)
	}
	raw := strings.TrimPrefix(req.URL.Path, "/")
	path, _ := filepath.Rel(".", raw) // remove that pesky leading slash
	if path == "tags" {
		query, err := url.ParseQuery(req.URL.RawQuery)
		if err != nil {
			w.WriteHeader(404)
			w.Write([]byte(fileNotFound))
			return
		}
		referer, _ := url.Parse(req.Referer())
		var crumbs *HTMLElement
		if referer.Hostname() == globalTree.Domain {
			refPath := strings.TrimPrefix(referer.Path, "/")
			refNode, err := globalTree.Search(refPath)
			if err == nil {
				crumbs = breadcrumbs(refNode, WWNavLink{Path: req.URL.String(), Text: "Tags"})
			}
		}
		page := buildTagListing(query, crumbs)
		headData := globalTree.GetDefaultHead()
		headData.Title = "Tags"
		buf, _ := buildDocument(page, *headData)
		w.Write(buf.Bytes())
		return
	}

	node, err := globalTree.Search(path)
	if err != nil {
		_, ok := os.Stat(filepath.Join(path, "wyweb"))
		if ok != nil {
			w.WriteHeader(404)
			w.Write([]byte(fileNotFound))
			return
		}
		log.Printf("Bizarro error bruv\n")
		return
	}
	meta := node.Data
	resolved := node.Resolved
	if node.Resolved.HTML == nil {
		switch (*meta).(type) {
		//case *WyWebRoot:
		case *WyWebListing:
			buildDirListing(node)
		case *WyWebPost:
			buildPost(node)
		case *WyWebGallery:
			buildGallery(node)
		default:
			fmt.Println("whoopsie")
			return
		}
	}
	buf, _ := buildDocument(node.Resolved.HTML, *globalTree.GetHeadData(meta, resolved))
	w.Write(buf.Bytes())
}

func WyWebStart(sockfile, group string) {
	defer os.Remove(sockfile)
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
				fmt.Printf("%s in use by %s\n", sockfile, out.String())
				os.Exit(2)
			}
		} else {
			break
		}
	}
	grp, err := user.LookupGroup(group)
	if err != nil {
		log.Printf("Could not find Specified group '%s'\n", group)
		return
	}
	gid, _ := strconv.Atoi(grp.Gid)
	if err = os.Chown(sockfile, -1, gid); err != nil {
		log.Printf("Failed to change ownership: %v\n", err)
		return
	}
	err = os.Chmod(sockfile, 0660)
	if err != nil {
		log.Printf("Could not change permissions for %s\n", sockfile)
		return
	}
	// Cleanup the sockfile.
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		os.Remove(sockfile)
		os.Exit(1)
	}()
	handler := WyWebHandler{}
	//	handler.tree = new(ConfigTree)
	http.Serve(socket, handler)
}

func main() {
	sock := flag.String("sock", "/tmp/wyweb.sock", "Path to the unix domain socket used by WyWeb")
	grp := flag.String("grp", "www-data", "Group of the unix domain socket used by WyWeb (Should be the accessible by your reverse proxy)")
	flag.Parse()
	log.SetFlags(log.Lshortfile)
	WyWebStart(*sock, *grp)
}
