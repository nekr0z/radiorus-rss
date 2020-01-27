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
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
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
)

func main() {
	flag.StringVar(&outputPath, "path", "./", "path to put resulting RSS file in")
	flag.StringVar(&programNumber, "brand", "57083", "brand number (defaults to Aerostat)")
	flag.Parse()

	programUrl := "http://www.radiorus.ru/brand/" + programNumber + "/episodes"

	page := getPage(programUrl)
	feed := &feeds.Feed{
		Link: &feeds.Link{Href: programUrl},
	}

	populateFeed(feed, page)
	describeFeed(feed)
	describeEpisodes(feed)

	feed.Created = time.Now()
	outputFile := outputPath + "radiorus-" + programNumber + ".rss"

	writeFeed(feed, outputFile)
}

func writeFeed(feed *feeds.Feed, filename string) {
	rss, err := feed.ToRss()
	if err != nil {
		log.Fatal(err)
	}
	output := []byte(rss)
	if err := ioutil.WriteFile(filename, output, 0644); err != nil {
		log.Fatal(err)
	}
}

func populateFeed(feed *feeds.Feed, page []byte) {
	for {
		feed.Title = string(programNameRe.FindSubmatch(page)[1])
		programImage := programImageRe.FindSubmatch(page)
		feed.Image = &feeds.Image{
			Link:  feed.Link.Href,
			Url:   string(programImage[2]),
			Title: string(programImage[4]),
		}

		episodes := episodeRe.FindAll(page, -1)

		badFeed := false

		for _, episode := range episodes {
			if len(episodeUrlRe.FindAllSubmatch(episode, -1)) > 1 {
				badFeed = true
				break
			}
			episodeUrl := "http://www.radiorus.ru/brand/" + string(episodeUrlRe.FindSubmatch(episode)[1])
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

		if badFeed {
			log.Println("Page looks strange. Episode in progress? Will wait for 15 minutes and try again...")
			time.Sleep(15 * 60 * time.Second)
			continue
		}
		break
	}
}

func describeFeed(feed *feeds.Feed) {
	programAboutUrl := strings.TrimSuffix(feed.Link.Href, "episodes") + "about"
	programAboutPage := getPage(programAboutUrl)
	programAbout := programAboutRe.FindSubmatch(programAboutPage)[1]
	re := regexp.MustCompile(`<(.+?)?>`)
	feed.Description = string(re.ReplaceAll(programAbout, []byte(``)))
}

func describeEpisodes(feed *feeds.Feed) {
	for _, item := range feed.Items {
		page := getPage(item.Link.Href)
		item.Description = describeEpisode(page)
	}
}

func describeEpisode(page []byte) string {
	return string(episodeDescRe.FindSubmatch(page)[1])
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
