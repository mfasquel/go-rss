package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"text/template"
	"unicode"

	"github.com/julienschmidt/httprouter"
)

var basePath string = os.Getenv("RSS_FEEDS_PATH")

type item struct {
	Title       string
	Link        string
	Description string
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
	tmpl, err := template.ParseFiles("feed.tmpl")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot parse template. %v", err)
		return ""
	}
	var buf bytes.Buffer
	tmpl.Execute(&buf, feed)
	return buf.String()
}

func (item *item) toRss() string {
	desc, err := base64.StdEncoding.DecodeString(item.Description)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot decode item %v description", item.Title)
		return ""
	}
	// TODO : template
	return "<item>" +
		"<title>" + item.Title + "</title>" +
		"<link>" + item.Link + "</link>" +
		"<description><![CDATA[" + string(desc) + "]]></description>" +
		"</item>"
}

func newItemFromPath(path string) (*item, error) {
	itemJSON, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Item file cannot be read for %v. %v\n", path, err)
		return nil, err
	}

	item := new(item)
	err = json.Unmarshal(itemJSON, item)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Item cannot be unmarshalled for %v. %v\n", path, err)
		return nil, err
	}

	return item, nil
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
		if name != "meta.json" {
			item, _ := newItemFromPath(path + "/" + name)
			// TODO error handling / best effort ?
			if item != nil {
				feed.Items = append(feed.Items, *item)
			}
		}
	}

	return feed, nil
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

	err = os.Mkdir(feedPath, 0755)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot create folder %v. %v\n", feedPath, err)
		http.Error(w, fmt.Sprintf("Cannot create feed %v", feedName), http.StatusInternalServerError)
		return
	}
	err = ioutil.WriteFile(feedPath+"/meta.json", buf.Bytes(), 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot create file %v. %v\n", feedPath+"/meta.json", err)
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
		fmt.Fprintf(os.Stderr, "Feed object creation failed for path %v. %v\n", feedPath, err)
		http.Error(w, fmt.Sprintf("Cannot load feed %v", feedName), http.StatusInternalServerError)
		return
	}

	acceptHeader := r.Header.Get("Accept")
	if acceptHeader == "application/json" {
		feedJSON, err := json.Marshal(feed)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Feed marshaling failed for path %v. %v\n", feedPath, err)
			http.Error(w, fmt.Sprintf("Cannot load feed %v", feedName), http.StatusInternalServerError)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		w.Write(feedJSON)
	} else {
		for i := range feed.Items {
			desc, err := base64.StdEncoding.DecodeString(feed.Items[i].Description)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Cannot decode item description %v. %v", feed.Items[i].Description, err)
				http.Error(w, fmt.Sprintf("Cannot load feed %v", feedName), http.StatusInternalServerError)
				return
			}
			feed.Items[i].Description = string(desc)
		}

		w.Header().Add("Content-Type", "application/rss+xml")
		fmt.Fprintf(w, feed.toRss())
	}
}

func listFeeds(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	infoList, err := ioutil.ReadDir(basePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot read dir %v. %v\n", basePath, err)
		http.Error(w, "Cannot list feeds", http.StatusInternalServerError)
		return
	}

	feedsMeta := make(map[string]feedMetaData)
	for _, info := range infoList {
		name := info.Name()
		// TODO : this also get all items, new function with only meta ?
		feed, _ := newFeedFromPath(basePath + "/" + name)
		if feed != nil {
			feedsMeta[name] = feed.MetaData
		}
	}

	resp, err := json.Marshal(feedsMeta)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot marshal feeds for base path %v. %v\n", basePath, err)
		http.Error(w, "Cannot list feeds", http.StatusInternalServerError)
		return
	}

	w.Header().Add("Content-Type", "application/json")
	w.Write(resp)
}

func createItem(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	feedName := params.ByName("feed")
	feedPath := basePath + "/" + feedName
	if _, err := os.Stat(feedPath); os.IsNotExist(err) {
		http.Error(w, fmt.Sprintf("The feed %v does not exist", feedName), http.StatusBadRequest)
		return
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(r.Body)
	var item item
	err := json.Unmarshal(buf.Bytes(), &item)
	if err != nil {
		http.Error(w, fmt.Sprintf("Cannot create item"), http.StatusBadRequest)
		return
	}

	var sanitizedTitle string
	for _, char := range item.Title {
		if unicode.IsDigit(char) || unicode.IsLetter(char) {
			sanitizedTitle += string(char)
		}
	}

	itemPath := feedPath + "/" + sanitizedTitle
	if _, err := os.Stat(itemPath); !os.IsNotExist(err) {
		http.Error(w, "Item already exist", http.StatusBadRequest)
		return
	}

	_, err = base64.StdEncoding.DecodeString(item.Description)
	if err != nil {
		http.Error(w, "Item description invalid", http.StatusBadRequest)
		return
	}

	err = ioutil.WriteFile(itemPath, buf.Bytes(), 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot create file %v. %v\n", itemPath, err)
		http.Error(w, "Cannot create item", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func getItem(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	feedName := params.ByName("feed")
	feedPath := basePath + "/" + feedName
	if _, err := os.Stat(feedPath); os.IsNotExist(err) {
		http.Error(w, fmt.Sprintf("The feed %v does not exist", feedName), http.StatusBadRequest)
		return
	}

	itemName := params.ByName("item")
	itemPath := feedPath + "/" + itemName
	if _, err := os.Stat(itemPath); os.IsNotExist(err) {
		http.Error(w, fmt.Sprintf("The item %v does not exist", itemName), http.StatusBadRequest)
		return
	}

	item, err := newItemFromPath(itemPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Item object creation failed for path %v. %v\n", itemPath, err)
		http.Error(w, fmt.Sprintf("Cannot load item %v", itemName), http.StatusInternalServerError)
		return
	}

	itemJSON, err := json.Marshal(item)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Item marshaling failed for path %v. %v\n", itemPath, err)
		http.Error(w, fmt.Sprintf("Cannot load item %v", itemName), http.StatusInternalServerError)
		return
	}

	w.Header().Add("Content-Type", "application/json")
	w.Write(itemJSON)
}

func listItems(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	feedName := params.ByName("feed")
	feedPath := basePath + "/" + feedName
	if _, err := os.Stat(feedPath); os.IsNotExist(err) {
		http.Error(w, fmt.Sprintf("The feed %v does not exist", feedName), http.StatusBadRequest)
		return
	}

	infoList, err := ioutil.ReadDir(feedPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot read dir %v. %v\n", feedPath, err)
		http.Error(w, "Cannot list items", http.StatusInternalServerError)
		return
	}

	items := make(map[string]item)
	for _, info := range infoList {
		name := info.Name()
		if name != "meta.json" {
			item, _ := newItemFromPath(feedPath + "/" + name)
			// TODO error handling / best effort ?
			if item != nil {
				items[name] = *item
			}
		}
	}

	resp, err := json.Marshal(items)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot marshal items for feed %v. %v\n", feedName, err)
		http.Error(w, "Cannot list items", http.StatusInternalServerError)
		return
	}

	w.Header().Add("Content-Type", "application/json")
	w.Write(resp)
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
	router.GET("/feeds/:feed/items/:item", getItem)

	log.Fatal(http.ListenAndServe(":8080", router))
}
