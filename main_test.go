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
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

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

func assertGolden(t *testing.T, actual []byte, golden string) {
	t.Helper()

	if *update {
		if _, err := os.Stat(golden); os.IsNotExist(err) {
			writeFile(actual, golden)
		} else {
			t.Log("file", golden, "exists, remove it to record new golden result")
		}
	}
	expected, err := ioutil.ReadFile(golden)
	if err != nil {
		t.Error("no file:", golden)
	}

	if !bytes.Equal(actual, expected) {
		t.Fatal("golden data doesn't match")
	}
}

func TestFeed(t *testing.T) {
	var page []byte

	feed := &feeds.Feed{
		Link: &feeds.Link{Href: "http://www.radiorus.ru/brand/57083/episodes"},
	}

	err := populateFeed(feed, page)
	assertStringContains(t, fmt.Sprint(err), "bad programme")

	page = helperLoadBytes(t, "episodes")
	page = cleanText(page)

	if err := populateFeed(feed, page); err != nil {
		t.Fatal(err)
	}

	page = helperLoadBytes(t, "about")
	page = cleanText(page)
	feed.Description, _ = processFeedDesc(page)

	actual := createFeed(feed)
	golden := filepath.Join("testdata", t.Name()+".golden")
	assertGolden(t, actual, golden)
}

func TestBadEpisode(t *testing.T) {
	feed := &feeds.Feed{
		Link: &feeds.Link{Href: "http://www.radiorus.ru/brand/57083/episodes"},
	}

	for i := 0; i <= 1; i++ {
		page := helperLoadBytes(t, "episodes.badep."+strconv.Itoa(i))
		page = cleanText(page)

		if err := populateFeed(feed, page); err != errBadEpisode {
			t.Error("for sample", i, "want:", errBadEpisode, "got:", err)
		}
	}
}

func TestNoImage(t *testing.T) {
	feed := &feeds.Feed{
		Link: &feeds.Link{Href: "http://www.radiorus.ru/brand/57083/episodes"},
	}

	page := helperLoadBytes(t, "episodes.noimg")
	page = cleanText(page)

	if err := populateFeed(feed, page); err != nil {
		t.Fatal(err)
	}

	actual := createFeed(feed)
	golden := filepath.Join("testdata", t.Name()+".golden")
	assertGolden(t, actual, golden)
}

func TestFindEpisodes(t *testing.T) {
	var tests = []string{
		"episodes",
		"episodes.59798",
	}

	for _, test := range tests {
		page := helperLoadBytes(t, test)
		page = cleanText(page)

		actual := bytes.Join(findEpisodes(page), []byte("\n&&&\n"))
		golden := filepath.Join("testdata", t.Name()+"."+test+".golden")
		assertGolden(t, actual, golden)
	}
}

func TestUpdatingFeed(t *testing.T) {
	var page []byte

	feed := &feeds.Feed{
		Link: &feeds.Link{Href: "http://www.radiorus.ru/brand/59798/episodes"},
	}

	page = helperLoadBytes(t, "episodes.59798")
	page = cleanText(page)

	if err := populateFeed(feed, page); err != nil {
		t.Fatal(err)
	}

	actual := createFeed(feed)
	golden := filepath.Join("testdata", t.Name()+".golden")
	assertGolden(t, actual, golden)
}

func TestPopulateFeed(t *testing.T) {
	var page []byte

	feed := &feeds.Feed{
		Link: &feeds.Link{Href: "https://smotrim.ru/brand/57083"},
	}

	page = helperLoadBytes(t, "smotrim.57083")
	page = cleanText(page)

	if err := populateFeed(feed, page); err != nil {
		t.Fatal(err)
	}

	actual := createFeed(feed)
	golden := filepath.Join("testdata", t.Name()+".golden")
	assertGolden(t, actual, golden)
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

func TestMissingFeed(t *testing.T) {
	server := helperMockServer(t)
	defer helperCleanupServer(t)

	if os.Getenv("DO_CRASH") == "1" {
		processURL(fmt.Sprintf("%s/brand/57084/episodes", server.URL))
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestMissingFeed")
	cmd.Env = append(os.Environ(), "DO_CRASH=1")
	out, err := cmd.CombinedOutput()
	if e, ok := err.(*exec.ExitError); !(ok && !e.Success()) {
		t.Fatalf("process ran with err %v, want exit status 1", err)
	}

	assertStringContains(t, string(out), "84/episodes: bad programme page")
}

func TestServedFeed(t *testing.T) {
	server := helperMockServer(t)
	defer helperCleanupServer(t)

	feed := processURL(fmt.Sprintf("%s/brand/57083/episodes", server.URL))

	actual := bytes.ReplaceAll(createFeed(feed), []byte(server.URL), []byte(fakeURL))
	golden := filepath.Join("testdata", t.Name()+".golden")
	assertGolden(t, actual, golden)
}

func BenchmarkServedFeed(b *testing.B) {
	server := helperMockServer(b)
	defer helperCleanupServer(b)

	for n := 0; n < b.N; n++ {
		processURL(fmt.Sprintf("%s/brand/57083/episodes", server.URL))
	}
}

func TestGetFeed(t *testing.T) {
	radiorus := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := helperLoadBytes(t, "episodes")
		_, _ = w.Write(page)
	}))
	defer radiorus.Close()

	smotrim := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := helperLoadBytes(t, "smotrim.57083")
		_, _ = w.Write(page)
	}))
	defer smotrim.Close()

	redir := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, smotrim.URL, 301)
	}))
	defer redir.Close()

	tests := map[string]struct {
		url  string
		want string
		desc bool
	}{
		"radiorus": {radiorus.URL, radiorus.URL, false},
		"smotrim":  {redir.URL, smotrim.URL, true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			feed := getFeed(tc.url)
			if tc.want != feed.Link.Href {
				t.Fatalf("\nwant %s, got %s", tc.want, feed.Link.Href)
			}
			ne := feed.Description != ""
			if ne != tc.desc {
				t.Fatalf("\nwant %v, got %v", tc.desc, ne)
			}
		})
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

func TestEpisodeID(t *testing.T) {
	type testval struct {
		url string
		id  string
	}

	var tests = []testval{
		{"http://www.radiorus.ru/brand/57083/episode/foo", "http://www.radiorus.ru/brand/57083/episode/foo"},
		{"https://www.radiorus.ru/brand/57083/episode/foo", "http://www.radiorus.ru/brand/57083/episode/foo"},
	}

	for _, test := range tests {
		got := episodeID(test.url)
		want := test.id
		if got != want {
			t.Error("want:", want, "got:", got)
		}
	}
}

func TestStripLink(t *testing.T) {
	type testval struct {
		raw string
		ret string
	}

	var tests = []testval{
		{`<a href="/brand/57083">"Аэростат"</a>`, `"Аэростат"`},
	}

	for _, test := range tests {
		got := stripLink(test.raw)
		want := test.ret
		if got != want {
			t.Error("want:", want, "got:", got)
		}
	}
}

func TestParseDate(t *testing.T) {
	type testval struct {
		b [][]byte
		d time.Time
	}

	var tests = []testval{
		{[][]byte{{}, []byte("24"), []byte("11"), []byte(`2019`), []byte("14"), []byte("10")}, time.Date(2019, time.November, 24, 14, 10, 0, 0, moscow)},
		{[][]byte{[]byte("foo"), []byte("bar"), []byte("baz"), []byte("qux"), []byte("none")}, time.Date(1970, time.January, 1, 0, 0, 0, 0, moscow)},
		{[][]byte{}, time.Date(1970, time.January, 1, 0, 0, 0, 0, moscow)},
	}

	for _, test := range tests {
		got := parseDate(test.b)
		want := test.d
		if !got.Equal(want) {
			t.Error("want:", want, "got:", got)
		}
	}
}

func TestParseErrors(t *testing.T) {
	type testval struct {
		src []byte
		re  *regexp.Regexp
		n   int
		err error
	}

	var tests = []testval{
		{
			[]byte("<h2>Аэростат</h2>"),
			programNameRe,
			1,
			nil,
		}, {
			[]byte("<h2>Аэростат</h2><h2>foo</h2>"),
			programNameRe,
			1,
			nil,
		}, {
			[]byte{},
			programNameRe,
			1,
			errCantParse,
		},
	}

	for _, test := range tests {
		res, got := parse(test.src, test.re, test.n)
		if test.err != got {
			t.Error("for", test.src, test.re, test.n, "\nwant:", test.err, "got:", got)
		}
		if test.n != len(res) {
			t.Error("for", test.src, test.re, test.n, "\nwant length:", test.n, "got:", len(res))
		}
	}
}

func TestProcessEpisodeDesc(t *testing.T) {
	page := helperLoadBytes(t, "blues")
	got, err := processEpisodeDesc(page)
	if err != nil {
		t.Fatal(err)
	}
	assertGolden(t, []byte(got), filepath.Join("testdata", "blues.golden"))
}
