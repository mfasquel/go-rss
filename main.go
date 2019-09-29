package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/julienschmidt/httprouter"
)

var basePath string = os.Getenv("RSS_FEEDS_PATH")

type item struct {
	Title       string
	Link        string
	Description string
	Date        string
}

type feedMetaData struct {
	Title       string
	Description string
	Link        string
}

type feed struct {
	MetaData feedMetaData
	Items    []item
}

func (feed *feed) toRss() string {
	// TODO : template + items
	return "<?xml version=\"1.0\" encoding=\"UTF-8\"?>" +
		"<rss version=\"2.0\">" +
		"<channel>" +
		"<title>" + feed.MetaData.Title + "</title>" +
		"<link>" + feed.MetaData.Link + "</link>" +
		"<description>" + feed.MetaData.Description + "</description>" +
		"</channel>" +
		"</rss>"
}

func newFeedFromPath(path string) (*feed, error) {
	metaJSON, err := ioutil.ReadFile(path + "/meta.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Metadata file cannot be read for %v. %v\n", path, err)
		return nil, err
	}
	infoList, err := ioutil.ReadDir(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot list items for %v. %v\n", path, err)
		return nil, err
	}

	feed := new(feed)
	err = json.Unmarshal(metaJSON, &feed.MetaData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Metadata cannot be unmarshalled for %v. %v\n", path, err)
		return nil, err
	}

	for _, info := range infoList {
		name := info.Name()
		if !strings.HasSuffix(name, "meta.json") {
			// TODO : items
		}
	}

	return feed, err
}

func createFeed(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	feedName := params.ByName("feed")
	feedPath := basePath + "/" + feedName

	if _, err := os.Stat(feedPath); !os.IsNotExist(err) {
		http.Error(w, fmt.Sprintf("Feed %v already exist", feedName), http.StatusBadRequest)
		return
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(r.Body)
	var metaData feedMetaData
	err := json.Unmarshal(buf.Bytes(), &metaData)
	if err != nil {
		http.Error(w, fmt.Sprintf("Cannot create feed %v, wrong metadata", feedName), http.StatusBadRequest)
		return
	}

	// TODO : review permissions
	err = os.Mkdir(feedPath, os.ModePerm)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot create feed %v. %v\n", feedPath, err)
		http.Error(w, fmt.Sprintf("Cannot create feed %v", feedName), http.StatusInternalServerError)
		return
	}
	err = ioutil.WriteFile(feedPath+"/meta.json", buf.Bytes(), os.ModePerm)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot create feed %v. %v\n", feedPath, err)
		http.Error(w, fmt.Sprintf("Cannot create feed %v", feedName), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func getFeed(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	feedName := params.ByName("feed")
	feedPath := basePath + "/" + feedName
	if _, err := os.Stat(feedPath); os.IsNotExist(err) {
		http.Error(w, fmt.Sprintf("The feed %v does not exist", feedName), http.StatusBadRequest)
		return
	}

	feed, err := newFeedFromPath(feedPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Cannot load feed %v", feedName), http.StatusInternalServerError)
		return
	}

	w.Header().Add("Content-Type", "application/json")
	fmt.Fprintf(w, feed.toRss())
}

func listFeeds(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	infoList, err := ioutil.ReadDir(basePath)
	if err != nil {
		http.Error(w, "Cannot list feeds", http.StatusInternalServerError)
		return
	}

	var names []string
	for _, info := range infoList {
		names = append(names, info.Name())
	}

	resp, err := json.Marshal(names)
	if err != nil {
		http.Error(w, "Cannot list feeds", http.StatusInternalServerError)
	}

	w.Header().Add("Content-Type", "application/json")
	w.Write(resp)
}

func createItem(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
}

func getItem(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
}

func listItems(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
}

func main() {
	if len(basePath) == 0 {
		basePath = "/data"
	}
	router := httprouter.New()
	router.GET("/feeds", listFeeds)
	router.POST("/feeds/:feed", createFeed)
	router.GET("/feeds/:feed", getFeed)
	router.POST("/feeds/:feed/items", createItem)
	router.GET("/feeds/:feed/items", listItems)
	router.POST("/feeds/:feed/items/:item", getItem)

	log.Fatal(http.ListenAndServe(":8080", router))
}
