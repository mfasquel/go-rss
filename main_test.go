package main

import (
	"testing"
)

func TestLoadFeed(t *testing.T) {
	if feed, _ := loadFeed(); feed != "Hello" {
		t.FailNow()
	}
}
