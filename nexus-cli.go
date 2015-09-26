package main

import (
	xml "encoding/xml"
	"flag"
	"fmt"
	"regexp"
	"strings"
	"time"

	logging "github.com/op/go-logging"
	gorequest "github.com/parnurzeal/gorequest"
)

var log = logging.MustGetLogger("example")

// https://golang.org/src/time/format.go
const timeLayout = "2006-01-02 15:04:05"
const timeLayoutShort = "2006-01-02"

const nexusTimeLayout = "2006-01-02 15:04:05.9 UTC"

func unmarshal(data []byte, query interface{}) {
	err := xml.Unmarshal(data, &query)
	if err != nil {
		panic(err)
	}
}

func makeRequest(uri string) []byte {
	log.Debug("URI:" + uri)

	request := gorequest.New()
	response, body, err := request.Get(uri).End()

	if err != nil {
		panic(err)
	}

	log.Debug(string(response.Status))

	data := []byte(body)
	return data
}

func listAllRepositories(host string) {

	type Repository struct {
		ResourceURI              string `xml:"resourceURI"`
		ContentResourceURI       string `xml:"contentResourceURI"`
		Name                     string `xml:"name"`
		Format                   string `xml:"format"`
		ID                       string `xml:"id"`
		EffectiveLocalStorageURL string `xml:"effectiveLocalStorageUrl"`
	}

	type Data struct {
		RepositoryList []Repository `xml:"repositories-item"`
	}

	type Query struct {
		XMLName xml.Name `xml:"repositories"`
		Name    string   `xml:"name"`
		Data    Data     `xml:"data"`
	}

	path := "service/local/all_repositories"

	data := makeRequest(host + path)
	q := Query{}
	unmarshal(data, &q)

	fmt.Printf("--== Found %d repositories ==-- \n", len(q.Data.RepositoryList))
	for _, repository := range q.Data.RepositoryList {
		fmt.Printf("%s;\t\t%s;\t%s\n", repository.ID, repository.Name, repository.Format)
	}
}

func deleteFactory(dryrun bool) func(string, string) {
	return func(uri, modified string) {
		log.Info("Deleting: " + uri)
		if !dryrun {
			request := gorequest.New()
			response, _, err := request.Delete(uri).End()
			if err != nil {
				panic(err)
			}
			log.Debug("Result (status): " + response.Status)
		} else {
			log.Debug("Nothign done dry-run")
		}
	}
}

func show(URI, modified string) {
	fmt.Printf("\t%s\t%s\n", modified, URI)
}

func find(uri, pattern string, olderThan time.Time, fn func(URI, modified string)) {

	type ContentItem struct {
		Text         string `xml:"text"`
		Leaf         bool   `xml:"leaf"`
		LastModified string `xml:"lastModified"`
		ResourceURI  string `xml:"resourceURI"`
	}

	type Data struct {
		XMLName         xml.Name      `xml:"data"`
		ContentItemList []ContentItem `xml:"content-item"`
	}

	type Query struct {
		XMLName xml.Name `xml:"content"`
		Data    Data     `xml:"data"`
	}

	data := makeRequest(uri)
	q := Query{}
	unmarshal(data, &q)

	for _, repository := range q.Data.ContentItemList {
		var validID = regexp.MustCompile(pattern)
		if validID.MatchString(repository.ResourceURI) {
			lastModified, err := time.Parse(nexusTimeLayout, repository.LastModified)
			if err != nil {
				panic(err)
			}
			if lastModified.Before(olderThan) {
				fn(repository.ResourceURI, repository.LastModified)
			}
		}
	}
}

func generateTime(timeString string) time.Time {
	timeTime, err := time.Parse(timeLayout, timeString)
	if err != nil {
		timeTime, err = time.Parse(timeLayoutShort, timeString)
		if err != nil {
			panic(err)
		}
	}
	return timeTime
}

func main() {
	listReposPtr := flag.Bool("repo-list", false, "List all repositories")

	showContentPtr := flag.Bool("list", false, "Show content of repository")

	deletePtr := flag.Bool("delete", false, "Delete content")

	dryRunPtr := flag.Bool("dry-run", true, "Dry run commands")

	var argHost string
	flag.StringVar(&argHost, "host", "localhost:8000", "Set nexus host")

	var argRepo string
	flag.StringVar(&argRepo, "repository", "jts-release", "Set nexus repository")

	var argPath string
	flag.StringVar(&argPath, "path", "", "Set path for artifact")

	var regexpPattern string
	flag.StringVar(&regexpPattern, "regexp", ".*", "Regexp search pattern")

	var beforeTime string
	flag.StringVar(&beforeTime, "before", time.Now().Format(timeLayout), "Act on on artifacts before date")

	flag.Parse()

	olderTime := generateTime(beforeTime)

	var operation func(string, string)

	if *showContentPtr {
		operation = show
	} else if *deletePtr {
		operation = deleteFactory(*dryRunPtr)
	}

	for _, hostName := range strings.Split(argHost, ",") {
		host := "http://" + hostName + "/nexus/"

		if *listReposPtr {
			listAllRepositories(host)
		} else if operation != nil {
			for _, repo := range strings.Split(argRepo, ",") {
				for _, path := range strings.Split(argPath, ",") {
					URI := host + "service/local/repositories/" + repo + "/content/" + path
					find(URI, regexpPattern, olderTime, operation)
				}
			}
		}
	}
}
