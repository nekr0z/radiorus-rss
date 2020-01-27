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
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/gorilla/feeds"
)

var update = flag.Bool("update", false, "update .golden files")

func helperLoadBytes(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("testdata", name)
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return bytes
}

func TestCleanText(t *testing.T) {
	about := helperLoadBytes(t, "about")
	actual := cleanText(about)
	golden := filepath.Join("testdata", t.Name()+".golden")
	if *update {
		if err := ioutil.WriteFile(golden, actual, 0644); err != nil {
			t.Fatal(err)
		}
	}
	expected, _ := ioutil.ReadFile(golden)

	if !bytes.Equal(actual, expected) {
		t.Fail()
	}
}

func TestFeed(t *testing.T) {
	page := helperLoadBytes(t, "episodes")
	page = cleanText(page)

	feed := &feeds.Feed{
		Link: &feeds.Link{Href: "http://www.radiorus.ru/brand/57083/episodes"},
	}

	populateFeed(feed, page)

	actual := createFeed(feed)
	golden := filepath.Join("testdata", t.Name()+".golden")
	if *update {
		if err := ioutil.WriteFile(golden, actual, 0644); err != nil {
			t.Fatal(err)
		}
	}
	expected, _ := ioutil.ReadFile(golden)

	if !bytes.Equal(actual, expected) {
		t.Fail()
	}
}
