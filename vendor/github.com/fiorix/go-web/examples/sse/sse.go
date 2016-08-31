// Copyright 2013-2014 The go-web authors.  All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
//
// This demo is live at http://cos.pe

package main

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/fiorix/go-web/httpxtra"
)

func main() {
	err := loadMovie("./ASCIImation.txt.gz")
	if err != nil {
		log.Println(err)
		return
	}
	http.HandleFunc("/", mainHandler)
	http.HandleFunc("/sse", sseHandler)
	s := http.Server{
		Addr:    ":8080",
		Handler: httpxtra.Handler{Logger: logger},
	}
	log.Fatal(s.ListenAndServe())
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./index.html")
}

func sseHandler(w http.ResponseWriter, r *http.Request) {
	conn, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Oops", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	sf := 0
	startFrame := r.FormValue("startFrame")
	if startFrame != "" {
		sf, _ = strconv.Atoi(startFrame)
	}
	if sf < 0 || sf >= cap(frames) {
		sf = 0
	}
	// Play the movie, frame by frame
	lw := w.(*httpxtra.LogWriter)
	for n, f := range frames[sf:] {
		_, err := fmt.Fprintf(w, "id: %d\ndata: %s\n\n", n+1, f.Buf)
		if err != nil {
			break // Client disconnected.
		}
		conn.Flush()
		if lw != nil {
			lw.Bytes += len(f.Buf)
		}
		time.Sleep(f.Time)
	}
}

func logger(r *http.Request, created time.Time, status, bytes int) {
	fmt.Println(httpxtra.ApacheCommonLog(r, created, status, bytes))
}

type Message struct {
	FrameNo  int
	FrameBuf string
}

type Frame struct {
	Time time.Duration
	Buf  string // This is a JSON-encoded Message{FrameBuf:...}
}

var frames []Frame

func loadMovie(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	gzfile, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	lineno := 1
	reader := bufio.NewReader(gzfile)
	frameNo := 1
	frameBuf := ""
	var (
		frameTime time.Duration
		part      string
	)
	for {
		if part, err = reader.ReadString('\n'); err != nil {
			break
		}

		switch lineno % 14 {
		case 0:
			b := html.EscapeString(frameBuf + part)
			j, _ := json.Marshal(Message{frameNo, b})
			frames = append(frames, Frame{frameTime, string(j)})
			frameNo++
			frameBuf = ""
		case 1:
			s := string(part)
			n, e := strconv.Atoi(s[:len(s)-1])
			if e == nil {
				frameTime = time.Duration(n) * time.Second / 10
			}
		default:
			frameBuf += part
		}
		lineno += 1
	}
	return nil
}
