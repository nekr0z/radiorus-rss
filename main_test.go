// Copyright (C) 2020 Evgeny Kuznetsov (evgeny@kuznetsov.md)
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gorilla/feeds"
)

var (
	update  = flag.Bool("update", false, "update .golden files")
	fakeURL = `**localhost**`
)

const pth = "testdata/brand/57083"

func helperLoadBytes(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("testdata", name)
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return bytes
}

func TestFeed(t *testing.T) {
	page := helperLoadBytes(t, "episodes")
	page = cleanText(page)

	feed := &feeds.Feed{
		Link: &feeds.Link{Href: "http://www.radiorus.ru/brand/57083/episodes"},
	}

	if err := populateFeed(feed, page); err != nil {
		t.Fatal(err)
	}

	page = helperLoadBytes(t, "about")
	page = cleanText(page)
	feed.Description = processFeedDesc(page)

	actual := createFeed(feed)
	golden := filepath.Join("testdata", t.Name()+".golden")
	if *update {
		writeFile(actual, golden)
	}
	expected, _ := ioutil.ReadFile(golden)

	if !bytes.Equal(actual, expected) {
		t.Fail()
	}
}

func TestServedFeed(t *testing.T) {
	server := helperMockServer(t)
	defer helperCleanupServer(t)

	feed := processURL(fmt.Sprintf("%s/brand/57083/episodes", server.URL))

	actual := bytes.ReplaceAll(createFeed(feed), []byte(server.URL), []byte(fakeURL))
	golden := filepath.Join("testdata", t.Name()+".golden")
	if *update {
		writeFile(actual, golden)
	}
	expected, _ := ioutil.ReadFile(golden)

	if !bytes.Equal(actual, expected) {
		t.Fail()
	}
}

func helperMockServer(t *testing.T) *httptest.Server {
	t.Helper()

	fileserver := http.FileServer(http.Dir("testdata"))
	server := httptest.NewServer(fileserver)

	episodes := helperLoadBytes(t, "episodes")
	writeFile(episodes, filepath.Join(pth, "episodes"))

	about := helperLoadBytes(t, "about")
	writeFile(about, filepath.Join(pth, "about"))

	return server
}

func helperCleanupServer(t *testing.T) {
	t.Helper()
	helperCleanupFile(t, "episodes")
	helperCleanupFile(t, "about")
}

func helperCleanupFile(t *testing.T, name string) {
	t.Helper()
	if err := os.Remove(filepath.Join(pth, name)); err != nil {
		t.Fatal(err)
	}
}

func TestEpisodeURLPrefix(t *testing.T) {
	url := "http://www.radiorus.ru/brand/57083/episodes"
	got := episodeURLPrefix(url)
	want := "http://www.radiorus.ru/brand/"

	if got != want {
		t.Fatal(fmt.Sprintf("got %v, want %v", got, want))
	}
}
