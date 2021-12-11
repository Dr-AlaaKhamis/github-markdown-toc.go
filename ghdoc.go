package ghtoc

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// GHToc GitHub TOC
type GHToc []string

// Print TOC to the console
func (toc *GHToc) Print() {
	for _, tocItem := range *toc {
		fmt.Println(tocItem)
	}
	fmt.Println()
}

// GHDoc GitHub document
type GHDoc struct {
	Path       string
	AbsPaths   bool
	StartDepth int
	Depth      int
	Escape     bool
	GhToken    string
	Indent     int
	Debug      bool
	html       string
}

// NewGHDoc create GHDoc
func NewGHDoc(Path string, AbsPaths bool, StartDepth int, Depth int, Escape bool, Token string, Indent int, Debug bool) *GHDoc {
	return &GHDoc{Path, AbsPaths, StartDepth, Depth, Escape, Token, Indent, Debug, ""}
}

func (doc *GHDoc) d(msg string) {
	if doc.Debug {
		log.Println(msg)
	}
}

// IsRemoteFile checks if path is for remote file or not
func (doc *GHDoc) IsRemoteFile() bool {
	u, err := url.Parse(doc.Path)
	if err != nil || u.Scheme == "" {
		doc.d("IsRemoteFile: false")
		return false
	}
	doc.d("IsRemoteFile: true")
	return true
}

// Convert2HTML downloads remote file
func (doc *GHDoc) Convert2HTML() error {
	doc.d("Convert2HTML: start.")
	defer doc.d("Convert2HTML: done.")

	if doc.IsRemoteFile() {
		htmlBody, ContentType, err := httpGet(doc.Path)
		doc.d("Convert2HTML: remote file. content-type: " + ContentType)
		if err != nil {
			return err
		}

		// if not a plain text - return the result (should be html)
		if strings.Split(ContentType, ";")[0] != "text/plain" {
			doc.html = string(htmlBody)
			return nil
		}

		// if remote file's content is a plain text
		// we need to convert it to html
		tmpfile, err := ioutil.TempFile("", "ghtoc-remote-txt")
		if err != nil {
			return err
		}
		defer tmpfile.Close()
		doc.Path = tmpfile.Name()
		if err = ioutil.WriteFile(tmpfile.Name(), htmlBody, 0644); err != nil {
			return err
		}
	}
	doc.d("Convert2HTML: local file: " + doc.Path)
	if _, err := os.Stat(doc.Path); os.IsNotExist(err) {
		return err
	}
	htmlBody, err := ConvertMd2Html(doc.Path, doc.GhToken)
	doc.d("Convert2HTML: converted to html, size: " + strconv.Itoa(len(htmlBody)))
	if err != nil {
		return err
	}
	if doc.Debug {
		htmlFile := doc.Path + ".debug.html"
		doc.d("Convert2HTML: write html file: " + htmlFile)
		if err := ioutil.WriteFile(htmlFile, []byte(htmlBody), 0644); err != nil {
			return err
		}
	}
	doc.html = htmlBody
	return nil
}

// GrabToc gets TOC from html
func (doc *GHDoc) GrabToc() *GHToc {
	doc.d("GrabToc: start, html size: " + strconv.Itoa(len(doc.html)))
	defer doc.d("GrabToc: done.")

	re := `(?si)<h(?P<num>[1-6])>\s*` +
		`<a\s*id="user-content-[^"]*"\s*class="anchor"\s*` +
		`href="(?P<href>[^"]*)"[^>]*>\s*` +
		`.*?</a>(?P<name>.*?)</h`
	r := regexp.MustCompile(re)
	listIndentation := generateListIndentation(doc.Indent)

	toc := GHToc{}
	minHeaderNum := 6
	var groups []map[string]string
	doc.d("GrabToc: matching ...")
	for idx, match := range r.FindAllStringSubmatch(doc.html, -1) {
		doc.d("GrabToc: match #" + strconv.Itoa(idx) + " ...")
		group := make(map[string]string)
		// fill map for groups
		for i, name := range r.SubexpNames() {
			if i == 0 || name == "" {
				continue
			}
			doc.d("GrabToc: process group: " + name + ": " + match[i] + " ...")
			group[name] = removeStuf(match[i])
		}
		// update minimum header number
		n, _ := strconv.Atoi(group["num"])
		if n < minHeaderNum {
			minHeaderNum = n
		}
		groups = append(groups, group)
	}

	var tmpSection string
	doc.d("GrabToc: processing groups ...")
	doc.d("Including starting from level " + strconv.Itoa(doc.StartDepth))
	for _, group := range groups {
		// format result
		n, _ := strconv.Atoi(group["num"])
		if n <= doc.StartDepth {
			continue
		}
		if doc.Depth > 0 && n > doc.Depth {
			continue
		}

		link, _ := url.QueryUnescape(group["href"])
		if doc.AbsPaths {
			link = doc.Path + link
		}

		tmpSection = removeStuf(group["name"])
		if doc.Escape {
			tmpSection = EscapeSpecChars(tmpSection)
		}
		tocItem := strings.Repeat(listIndentation(), n-minHeaderNum-doc.StartDepth) + "* " +
			"[" + tmpSection + "]" +
			"(" + link + ")"
		//fmt.Println(tocItem)
		toc = append(toc, tocItem)
	}

	return &toc
}

// GetToc return GHToc for a document
func (doc *GHDoc) GetToc() *GHToc {
	if err := doc.Convert2HTML(); err != nil {
		log.Fatal(err)
		return nil
	}
	return doc.GrabToc()
}
