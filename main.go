// Copyright (C) 2019-2020 Evgeny Kuznetsov (evgeny@kuznetsov.md)
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
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/feeds"
)

type subst struct {
	from string
	to   string
}

var (
	substitutes = []subst{ // these need to be changed to show up properly in the feed
		{from: `&quot;`, to: `"`},
		{from: `&ndash;`, to: `–`},
	}

	programNameRe  = regexp.MustCompile(`<h2>(.+?)?</h2>`)
	programAboutRe = regexp.MustCompile(`(?s)<div class="brand__content_text__anons">(.+?)?</div>`)
	programImageRe = regexp.MustCompile(`(?s)<div class="brand\-promo__header">(.+?)?<img src="(.+?)?"(.+?)?alt='(.+?)?'>`)
	episodeRe      = regexp.MustCompile(`(?s)<div class="brand__list\-\-wrap\-\-item">(.+?)?<div class="add\-to\-list">`)
	episodeAudioRe = regexp.MustCompile(`data\-id="(.+?)?">`)
	episodeDateRe  = regexp.MustCompile(`brand\-time brand\-menu\-link">(.+?)?\.(.+?)?\.(.+?)? в (.+?)?:(.+?)?</a>`)
	episodeDescRe  = regexp.MustCompile(`<p class="anons">(.+?)?</p>`)
	episodeTitleRe = regexp.MustCompile(`title brand\-menu\-link">(.+?)?</a>`)
	episodeUrlRe   = regexp.MustCompile(`<a href="/brand/(.+?)?" class="title`)

	outputPath, programNumber string

	errBadEpisode = fmt.Errorf("bad episode")
	errCantParse  = fmt.Errorf("could not parse page")
)

func main() {
	flag.StringVar(&outputPath, "path", "./", "path to put resulting RSS file in")
	flag.StringVar(&programNumber, "brand", "57083", "brand number (defaults to Aerostat)")
	flag.Parse()

	url := "http://www.radiorus.ru/brand/" + programNumber + "/episodes"

	feed := processURL(url)

	feed.Created = time.Now()
	output := createFeed(feed)
	outputFile := outputPath + "radiorus-" + programNumber + ".rss"

	writeFile(output, outputFile)
}

func processURL(url string) *feeds.Feed {
	feed := getFeed(url)

	var wg sync.WaitGroup
	wg.Add(1)
	go describeFeed(feed, &wg)
	describeEpisodes(feed)
	wg.Wait()

	return feed
}

func createFeed(feed *feeds.Feed) []byte {
	rss, err := feed.ToRss()
	if err != nil {
		log.Fatal(err)
	}
	return []byte(rss)
}

func writeFile(output []byte, filename string) {
	if err := ioutil.WriteFile(filename, output, 0644); err != nil {
		log.Fatal(err)
	}
}

func getFeed(url string) (feed *feeds.Feed) {
	feed = &feeds.Feed{
		Link: &feeds.Link{Href: url},
	}

	for {
		page := getPage(url)
		if err := populateFeed(feed, page); err == errBadEpisode {
			time.Sleep(15 * 60 * time.Second)
			continue
		} else if err != nil {
			err = fmt.Errorf("could not process %v: %w", url, err)
			log.Fatal(err)
		}
		break
	}

	return feed
}

func populateFeed(feed *feeds.Feed, page []byte) (err error) {
	titleMatch := programNameRe.FindSubmatch(page)
	if len(titleMatch) < 1 {
		return fmt.Errorf("bad program page")
	}

	feed.Title = string(titleMatch[1])
	programImage := programImageRe.FindSubmatch(page)
	feed.Image = &feeds.Image{
		Link:  feed.Link.Href,
		Url:   string(programImage[2]),
		Title: string(programImage[4]),
	}

	episodes := episodeRe.FindAll(page, -1)
	urlPrefix := episodeURLPrefix(feed.Link.Href)

	for _, episode := range episodes {
		if len(episodeUrlRe.FindAllSubmatch(episode, -1)) > 1 {
			return errBadEpisode
		}
		episodeUrl := urlPrefix + string(episodeUrlRe.FindSubmatch(episode)[1])
		episodeTitle := string(episodeTitleRe.FindSubmatch(episode)[1])
		episodeAudioUrl := "https://audio.vgtrk.com/download?id=" + string(episodeAudioRe.FindSubmatch(episode)[1])
		dateBytes := episodeDateRe.FindSubmatch(episode)
		var date [5]int
		for i, b := range dateBytes[1:] {
			d, err := strconv.Atoi(string(b))
			if err != nil {
				log.Fatal(err)
			}
			date[i] = d
		}
		moscow := time.FixedZone("Moscow Time", int((3 * time.Hour).Seconds()))
		episodeDate := time.Date(date[2], time.Month(date[1]), date[0], date[3], date[4], 0, 0, moscow)

		feed.Add(&feeds.Item{
			Id:    episodeUrl,
			Link:  &feeds.Link{Href: episodeUrl},
			Title: episodeTitle,
			Enclosure: &feeds.Enclosure{
				Url:    episodeAudioUrl,
				Length: "1024",
				Type:   "audio/mpeg",
			},
			Created: episodeDate,
		})
	}
	return nil
}

func describeFeed(feed *feeds.Feed, wg *sync.WaitGroup) {
	defer wg.Done()
	url := strings.TrimSuffix(feed.Link.Href, "episodes") + "about"
	page := getPage(url)
	desc, err := processFeedDesc(page)
	if err != nil {
		log.Printf("could not find programme description on page %v: %v", url, err)
	}
	feed.Description = desc
}

func processFeedDesc(page []byte) (string, error) {
	matches := programAboutRe.FindSubmatch(page)
	if len(matches) < 2 {
		return "", errCantParse
	}
	re := regexp.MustCompile(`<(.+?)?>`)
	return string(re.ReplaceAll(matches[1], []byte(``))), nil
}

func describeEpisodes(feed *feeds.Feed) {
	var wg sync.WaitGroup
	for _, item := range feed.Items {
		wg.Add(1)
		go describeEpisode(item, &wg)
	}
	wg.Wait()
}

func describeEpisode(item *feeds.Item, wg *sync.WaitGroup) {
	defer wg.Done()
	page := getPage(item.Link.Href)
	desc, err := processEpisodeDesc(page)
	if err != nil {
		log.Printf("could not find episode description on page %v: %v", item.Link.Href, err)
	}
	item.Description = desc
}

func processEpisodeDesc(page []byte) (string, error) {
	matches := episodeDescRe.FindSubmatch(page)
	if len(matches) < 2 {
		return "", errCantParse
	}
	return string(matches[1]), nil
}

func getPage(pageUrl string) []byte {
	res, err := http.Get(pageUrl)
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()
	page, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Fatal(err)
	}

	page = cleanText(page)

	return page
}

// cleanText replaces HTML-encoded symbols with proper UTF
func cleanText(b []byte) []byte {
	for _, sub := range substitutes {
		re := regexp.MustCompile(sub.from)
		b = re.ReplaceAll(b, []byte(sub.to))
	}
	return b
}

// episodeURLPrefix derives common episode URL prefix from programme page URL
func episodeURLPrefix(url string) string {
	return strings.Split(url, "/brand/")[0] + "/brand/"
}
