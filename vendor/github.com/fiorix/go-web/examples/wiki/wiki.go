// Copyright 2013-2014 The go-web authors.  All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/fiorix/go-web/httpxtra"
	"github.com/fiorix/go-web/urlparams"
)

const (
	pagesDir    = "./pages"
	pagesDirLen = len(pagesDir) - 1
	extLen      = len(".txt")
)

func main() {
	http.HandleFunc("/", IndexHandler)
	http.HandleFunc("/view/", viewHandler)
	http.HandleFunc("/edit/", editHandler)
	http.HandleFunc("/save/", saveHandler)
	handler := httpxtra.Handler{
		Logger:  logger,
		Handler: http.DefaultServeMux, // default
	}
	s := http.Server{
		Addr:    ":8080",
		Handler: handler,
	}
	log.Fatal(s.ListenAndServe())
}

func logger(r *http.Request, created time.Time, status, bytes int) {
	fmt.Println(httpxtra.ApacheCommonLog(r, created, status, bytes))
}

var templates = template.Must(template.ParseGlob("./templates/*.html"))

func renderTemplate(w http.ResponseWriter, name string, a interface{}) error {
	err := templates.ExecuteTemplate(w, name, a)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, http.StatusText(500), 500)
	}
	return err
}

type Page struct {
	Title string
	Body  []byte
}

func (p *Page) save() error {
	filename := filepath.Join(pagesDir, p.Title+".txt")
	return ioutil.WriteFile(filename, p.Body, 0600)
}

func loadPage(title string) (*Page, error) {
	filename := filepath.Join(pagesDir, title+".txt")
	body, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return &Page{Title: title, Body: body}, nil
}

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	files, err := filepath.Glob(filepath.Join(pagesDir, "*.txt"))
	if err != nil {
		log.Println(err.Error())
		http.Error(w, http.StatusText(500), 500)
		return
	}
	type PageList struct{ Title string }
	pages := make([]PageList, len(files))
	for n, name := range files {
		v := name[pagesDirLen:]
		pages[n].Title = v[:len(v)-extLen]
	}
	renderTemplate(w, "index.html", map[string]interface{}{"Pages": pages})
}

func viewHandler(w http.ResponseWriter, r *http.Request) {
	vars := urlparams.Parse("/view/:title", r.URL.Path)
	p, err := loadPage(vars["title"])
	if err != nil {
		http.Redirect(w, r, "/edit/"+vars["title"], http.StatusFound)
		return
	}
	renderTemplate(w, "view.html", p)
}

func editHandler(w http.ResponseWriter, r *http.Request) {
	vars := urlparams.Parse("/edit/:title", r.URL.Path)
	p, err := loadPage(vars["title"])
	if err != nil {
		p = &Page{Title: vars["title"]}
	}
	renderTemplate(w, "edit.html", p)
}

func saveHandler(w http.ResponseWriter, r *http.Request) {
	vars := urlparams.Parse("/save/:title", r.URL.Path)
	body := r.FormValue("body")
	p := &Page{Title: vars["title"], Body: []byte(body)}
	err := p.save()
	if err != nil {
		log.Println(err.Error())
		http.Error(w, http.StatusText(500), 500)
		return
	}
	http.Redirect(w, r, "/view/"+vars["title"], http.StatusFound)
}
