package main

import (
	"bytes"
	_ "embed"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"

	. "wyweb.site/internal/wyweb"
	"wyweb.site/util"
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
	crumbs, bcsd := Breadcrumbs(node, WWNavLink{Path: strings.TrimPrefix(req.URL.String(), "/"), Text: "Tags"})
	page := BuildTagListing(node, taglist, crumbs)
	headData := node.Tree.GetDefaultHead()
	headData.Title = "Tags"
	buf, _ := BuildDocument(page, *headData, bcsd)
	w.Write(buf.Bytes())
}

func RouteStatic(node *ConfigNode, w http.ResponseWriter) {
	var err error
	var structuredData []string
	if node.HTML == nil {
		switch node.NodeKind {
		//case *WyWebRoot:
		case WWLISTING:
			structuredData, err = BuildDirListing(node)
		case WWPOST:
			structuredData, err = BuildPost(node)
		case WWGALLERY:
			structuredData, err = BuildGallery(node)
		default:
			w.WriteHeader(500)
			return
		}
		node.HTML.Append(BuildFooter(node))
		log.Println(structuredData)
	}
	if err != nil {
		w.WriteHeader(404)
		w.Write([]byte(fileNotFound))
	}
	buf, _ := BuildDocument(node.HTML, *node.GetHTMLHeadData(), structuredData...)
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
	defer util.Timer(fmt.Sprintf("%s: %s requested %s", GetHost(req), GetRemoteAddr(req), req.RequestURI))()
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
