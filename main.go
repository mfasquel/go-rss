package main

import (
	"log"
	"net/http"
	"os"

	"github.com/julienschmidt/httprouter"
)

var basePath string = os.Getenv("RSS_FEEDS_PATH")

type item struct {
	title       string
	link        string
	description string
}

type feedMetaData struct {
	title       string
	description string
	link        string
}

type feed struct {
	metaData feedMetaData
	items    []item
}

func loadFeed() (string, error) {
	return "Hello", nil
}

func createFeed(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
}

func getFeed(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
}

func listFeeds(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
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
	router.POST("/feeds", createFeed)
	router.GET("/feeds/:feed", getFeed)
	router.POST("/feeds/:feed/items", createItem)
	router.GET("/feeds/:feed/items", listItems)
	router.POST("/feeds/:feed/items/:item", getItem)

	log.Fatal(http.ListenAndServe(":8080", router))
}
