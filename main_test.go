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
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/gorilla/feeds"
)

var (
	update  = flag.Bool("update", false, "update .golden files")
	fakeURL = `**localhost**`
)

const pth = "testdata/brand/57083"

func helperLoadBytes(t testing.TB, name string) []byte {
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
	feed.Description, _ = processFeedDesc(page)

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

func TestMissingEpisode(t *testing.T) {
	server := helperMockServer(t)
	defer helperCleanupServer(t)

	item := feeds.Item{
		Id:   "aabb",
		Link: &feeds.Link{Href: fmt.Sprintf("%s/brand/none", server.URL)},
	}

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	var wg sync.WaitGroup
	wg.Add(1)
	describeEpisode(&item, &wg)

	assertStringContains(t, buf.String(), fmt.Sprintf("could not find episode description on page %v: %v", item.Link.Href, errCantParse))
}

func assertStringContains(t *testing.T, got, want string) {
	if !strings.Contains(got, want) {
		t.Fatalf("%v does not contain %v", got, want)
	}
}

func TestMissingFeedDesc(t *testing.T) {
	server := helperMockServer(t)
	defer helperCleanupFile(t, "episodes")
	helperCleanupFile(t, "about")

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	processURL(fmt.Sprintf("%s/brand/57083/episodes", server.URL))

	assertStringContains(t, buf.String(), fmt.Sprintf("could not find programme description on page %v: %v", server.URL+"/brand/57083/about", errCantParse))
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

func BenchmarkServedFeed(b *testing.B) {
	server := helperMockServer(b)
	defer helperCleanupServer(b)

	for n := 0; n < b.N; n++ {
		processURL(fmt.Sprintf("%s/brand/57083/episodes", server.URL))
	}
}

func helperMockServer(t testing.TB) *httptest.Server {
	t.Helper()

	fileserver := http.FileServer(http.Dir("testdata"))
	server := httptest.NewServer(fileserver)

	episodes := helperLoadBytes(t, "episodes")
	writeFile(episodes, filepath.Join(pth, "episodes"))

	about := helperLoadBytes(t, "about")
	writeFile(about, filepath.Join(pth, "about"))

	return server
}

func helperCleanupServer(t testing.TB) {
	t.Helper()
	helperCleanupFile(t, "episodes")
	helperCleanupFile(t, "about")
}

func helperCleanupFile(t testing.TB, name string) {
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
		t.Fatalf("got %v, want %v", got, want)
	}
}
