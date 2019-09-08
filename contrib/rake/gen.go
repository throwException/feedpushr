// +build ignore

package main

import (
	"bufio"
	"log"
	"os"
	"strings"
	"text/template"
	"time"
)

// This program generates stopwords.go.
// It can be invoked by running go generate

var srcTemplate = template.Must(template.New("").Parse(`
// Code generated by go generate; DO NOT EDIT.
// This file was generated at {{ .Timestamp }} using data from {{ .Filename }}
package main

var stopWords = []string{
{{- range .Words }}
	{{ printf "%q" . }},
{{- end }}
}
`))

func main() {
	filename := "./SmartStoplist-en.txt"
	f1, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer f1.Close()

	sc := bufio.NewScanner(f1)
	words := []string{}

	for sc.Scan() {
		if !strings.HasPrefix(sc.Text(), "#") {
			words = append(words, sc.Text())
		}
	}

	if sc.Err() != nil {
		log.Fatal(sc.Err())
	}

	f2, err := os.Create("stopwords.go")
	if err != nil {
		log.Fatal(err)
	}
	defer f2.Close()

	srcTemplate.Execute(f2, struct {
		Timestamp time.Time
		Filename  string
		Words     []string
	}{
		Timestamp: time.Now(),
		Filename:  filename,
		Words:     words,
	})
}