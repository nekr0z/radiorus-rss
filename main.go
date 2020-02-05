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
	episodeDescRe  = regexp.MustCompile(`<p class="anons">(.+?)?</p>`)
	episodeTitleRe = regexp.MustCompile(`title brand\-menu\-link">(.+?)?</a>`)
	episodeUrlRe   = regexp.MustCompile(`<a href="/brand/(.+?)?" class="title`)

	outputPath, programNumber string

	errBadEpisode = fmt.Errorf("bad episode")
	errCantParse  = fmt.Errorf("could not parse page")

	moscow = time.FixedZone("Moscow Time", int((3 * time.Hour).Seconds()))
)

func main() {
	flag.StringVar(&outputPath, "path", "./", "path to put resulting RSS file in")
	flag.StringVar(&programNumber, "brand", "57083", "brand number (defaults to Aerostat)")
	flag.Parse()

	url := "https://www.radiorus.ru/brand/" + programNumber + "/episodes"

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

	page := getPage(url)
	if err := populateFeed(feed, page); err != nil {
		err = fmt.Errorf("could not process %v: %w", url, err)
		log.Fatal(err)
	}

	return feed
}

func populateFeed(feed *feeds.Feed, page []byte) (err error) {
	titleMatch := programNameRe.FindSubmatch(page)
	if len(titleMatch) < 1 {
		return fmt.Errorf("bad programme page")
	}

	feed.Title = stripLink(string(titleMatch[1]))
	programImage := programImageRe.FindSubmatch(page)
	feed.Image = &feeds.Image{
		Link:  feed.Link.Href,
		Url:   string(programImage[2]),
		Title: string(programImage[4]),
	}

	episodes := findEpisodes(page)
	urlPrefix := episodeURLPrefix(feed.Link.Href)

	for _, episode := range episodes {
		if len(episodeUrlRe.FindAllSubmatch(episode, -1)) > 1 {
			return errBadEpisode
		}
		episodeUrl := urlPrefix + string(episodeUrlRe.FindSubmatch(episode)[1])
		episodeTitle := string(episodeTitleRe.FindSubmatch(episode)[1])
		enclosure := findEnclosure(episode)
		date := findDate(episode)

		feed.Add(&feeds.Item{
			Id:        episodeID(episodeUrl),
			Link:      &feeds.Link{Href: episodeUrl},
			Title:     episodeTitle,
			Enclosure: enclosure,
			Created:   date,
		})
	}
	return nil
}

func findDate(ep []byte) time.Time {
	episodeDateRe := regexp.MustCompile(`brand\-time brand\-menu\-link">(.+?)?\.(.+?)?\.(.+?)? в (.+?)?:(.+?)?</a>`)
	dateBytes := episodeDateRe.FindSubmatch(ep)
	return parseDate(dateBytes)
}

func parseDate(bytes [][]byte) time.Time {
	if len(bytes) < 4 {
		return time.Date(1970, time.January, 1, 0, 0, 0, 0, moscow)
	}

	var date [5]int
	for i, b := range bytes[1:] {
		d, err := strconv.Atoi(string(b))
		if err != nil {
			return time.Date(1970, time.January, 1, 0, 0, 0, 0, moscow)
		}
		date[i] = d
	}
	return time.Date(date[2], time.Month(date[1]), date[0], date[3], date[4], 0, 0, moscow)
}

func findEnclosure(ep []byte) *feeds.Enclosure {
	re := regexp.MustCompile(`data\-type="audio"\s+data\-id="(.+?)?">`)

	matches := re.FindSubmatch(ep)
	if len(matches) < 2 {
		return &feeds.Enclosure{}
	}

	url := "https://audio.vgtrk.com/download?id=" + string(matches[1])

	return &feeds.Enclosure{
		Url:    url,
		Length: "1024",
		Type:   "audio/mpeg",
	}
}

func findEpisodes(page []byte) [][]byte {
	episodeRe := regexp.MustCompile(`(?s)<div class="brand__list\-\-wrap\-\-item">(.+?)?data-id="(.+?)"></div>`)
	episodes := episodeRe.FindAll(page, -1)
	return episodes
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

// episodeID generates episode ID from episode URL,
// changes "https://" to "http://" for backwards compatibility purposes
func episodeID(url string) string {
	if strings.HasPrefix(url, "https://") {
		return "http://" + strings.TrimPrefix(url, "https://")
	}
	return url
}

// stripLink strips string of <a> tags
func stripLink(s string) string {
	re := regexp.MustCompile(`</?a.*?>`)
	return re.ReplaceAllString(s, "")
}
