# radiorus-rss
инструмент для создания RSS-лент передач «Радио России»

[![Build Status](https://travis-ci.org/nekr0z/radiorus-rss.svg?branch=master)](https://travis-ci.org/nekr0z/radiorus-rss) [![codecov](https://codecov.io/gh/nekr0z/radiorus-rss/branch/master/graph/badge.svg)](https://codecov.io/gh/nekr0z/radiorus-rss) [![Go Report Card](https://goreportcard.com/badge/github.com/nekr0z/radiorus-rss)](https://goreportcard.com/report/github.com/nekr0z/radiorus-rss) [![GolangCI](https://golangci.com/badges/github.com/nekr0z/radiorus-rss.svg)](https://golangci.com)

Этот парсер можно использовать для преобразования страницы передачи на сайте «Радио России» в RSS-ленту подкаста. На сегодняшний день поддерживаются только аудиопередачи, при попытке использовать идентификатор передачи с видеовыпусками лента будет сгенерирована, но в ней не будет прямых ссылок на видеофайлы.

## Использование
Может работать в качестве скрипта (при установленном `Go`) или в скомпилированном виде как приложение.

### Без компиляции
```
$ go run main.go [опции]
```

### Как приложение
> Необходимо предварительно скомпилировать через `go build`.
```
$ radiorus-rss [опции]
```

### Опции
```
-brand XXXXX
```
выбор передачи. Здесь `XXXXX` — число, как правило, пятизначное, которое можно получить из URL страницы на сайте «Радио России». Так, страница передачи «Мы очень любим оперу» имеет URL вида `www.radiorus.ru/brand/59798/about` — значит, для этой передачи `XXXXX` — `59798`. По умолчанию используется передача `57083` — «Аэростат» Бориса Гребенщикова.

```
-path [путь]
```
путь, где будет создан файл с RSS-лентой. По умолчанию — текущая директория.

## Применение
Один из возможных сценариев использования — загрузить скомпилированное приложение на сервер и настроить автоматическое создание RSS-ленты через `cron` (промежутки подобрать сообразно с частотой выхода передачи). Именно так сделана [RSS-лента для передачи «Аэростат»](http://evgenykuznetsov.org/feeds/radiorus-57083.rss) на моём сайте.

## При создании использованы
(и при компиляции входят в состав приложения):
* [gorilla/feeds](https://github.com/gorilla/feeds) Copyright © 2013-2018 The Gorilla Feeds Authors
* [The Go Programming Language](https://golang.org) Copyright © 2009 The Go Authors

## Если нравится и хочется помочь
Можно открыть issue или pull request, а можно просто [купить мне кофе](https://www.buymeacoffee.com/nekr0z).
