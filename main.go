package main

import (
	"flag"
	"github.com/gorilla/feeds"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"time"
)

type subst struct {
	from string
	to   string
}

var (
	substitutes = []subst{
		{from: `&quot;`, to: `"`},
		{from: `&ndash;`, to: `–`},
	}

	programNameRe  = regexp.MustCompile(`<h2>(.+?)?</h2>`)
	episodeRe      = regexp.MustCompile(`(?s)<div class="brand__list\-\-wrap\-\-item">(.+?)?<div class="add\-to\-list">`)
	episodeAudioRe = regexp.MustCompile(`data\-id="(.+?)?">`)
	episodeDateRe  = regexp.MustCompile(`brand\-time brand\-menu\-link">(.+?)?\.(.+?)?\.(.+?)? в (.+?)?:(.+?)?</a>`)
	episodeDescRe  = regexp.MustCompile(`<p class="anons">(.+?)?</p>`)
	episodeTitleRe = regexp.MustCompile(`title brand\-menu\-link">(.+?)?</a>`)
	episodeUrlRe   = regexp.MustCompile(`<a href="/brand/(.+?)?" class="title`)

	feed = &feeds.Feed{
		Created: time.Now(),
	}
	outputPath, programNumber string
	err                       error
)

func main() {
	flag.StringVar(&outputPath, "path", "./", "path to put resulting RSS file in")
	flag.StringVar(&programNumber, "brand", "57083", "brand number (defaults to Aerostat)")
	flag.Parse()

	programUrl := "http://www.radiorus.ru/brand/" + programNumber + "/episodes"

	programPage := getPage(programUrl)

	for _, sub := range substitutes {
		re := regexp.MustCompile(sub.from)
		programPage = re.ReplaceAll(programPage, []byte(sub.to))
	}

	feed.Title = string(programNameRe.FindSubmatch(programPage)[1])
	feed.Link = &feeds.Link{Href: programUrl}
	episodes := episodeRe.FindAll(programPage, -1)

	for _, episode := range episodes {
		episodeUrl := "http://www.radiorus.ru/brand/" + string(episodeUrlRe.FindSubmatch(episode)[1])
		episodeTitle := string(episodeTitleRe.FindSubmatch(episode)[1])
		episodeAudioUrl := "https://audio.vgtrk.com/download?id=" + string(episodeAudioRe.FindSubmatch(episode)[1])
		dateBytes := episodeDateRe.FindSubmatch(episode)
		var date [5]int
		for i, b := range dateBytes[1:] {
			date[i], err = strconv.Atoi(string(b))
			if err != nil {
				log.Fatal(err)
			}
		}
		moscow := time.FixedZone("Moscow Time", int((3 * time.Hour).Seconds()))
		episodeDate := time.Date(date[2], time.Month(date[1]), date[0], date[3], date[4], 0, 0, moscow)

		episodePage := getPage(episodeUrl)
		episodeDesc := string(episodeDescRe.FindSubmatch(episodePage)[1])

		feed.Add(&feeds.Item{
			Id:    episodeUrl,
			Link:  &feeds.Link{Href: episodeUrl},
			Title: episodeTitle,
			Enclosure: &feeds.Enclosure{
				Url:    episodeAudioUrl,
				Length: "1024",
				Type:   "audio/mpeg",
			},
			Created:     episodeDate,
			Description: episodeDesc,
		})
	}

	rss, err := feed.ToRss()
	if err != nil {
		log.Fatal(err)
	}
	outputFile := outputPath + "radiorus-" + programNumber + ".rss"
	output := []byte(rss)
	if err := ioutil.WriteFile(outputFile, output, 0644); err != nil {
		log.Fatal(err)
	}
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
	return page
}
