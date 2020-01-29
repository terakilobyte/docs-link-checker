package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
	"mvdan.cc/xurls/v2"
)

type Check struct {
	Line    int    `json:"line"`
	Source  string `json:"source"`
	Message string `json:"message"`
	Ok      bool   `json:"ok"`
}

const (
	TLSGIT = "https://github.com/"
	GIT    = "http://github.com/"
)

func SplitOrgAndRepo(url string) []string {
	if strings.HasPrefix(url, TLSGIT) {
		return strings.Split(strings.TrimPrefix(url, TLSGIT), "/")
	}
	return strings.Split(strings.TrimPrefix(url, GIT), "/")
}

func CheckGitHub(line int, org, repo string, ch chan<- Check, gh *github.Client, wg *sync.WaitGroup) {
	to := make(chan Check, 1)
	defer wg.Done()
	c := Check{Source: fmt.Sprintf("%s/%s", org, repo), Line: line}
	go func() {
		stats, resp, err := gh.Repositories.Get(context.Background(), org, repo)
		if err != nil {
			s := fmt.Sprintf("err: %q", err)
			c.Message = s
			to <- c
			return
		}
		if resp.StatusCode == http.StatusOK {
			if stats.PushedAt.AddDate(1, 0, 0).Before(time.Now()) {
				s := "stale (last commit more than a year ago)"
				c.Message = s
				to <- c
				return

			}
			c.Ok = true
			to <- c
		} else {
			s := fmt.Sprintf("got status code %d", resp.StatusCode)
			c.Message = s
			to <- c
		}
	}()
	select {
	case res := <-to:
		ch <- res
	case <-time.After(5 * time.Second):
		s := "timeout (5 seconds)"
		c.Message = s
		ch <- c
	}
}

func CheckLink(line int, url string, ch chan<- Check, cl *http.Client, wg *sync.WaitGroup) {
	to := make(chan Check, 1)
	defer wg.Done()
	c := Check{Source: url, Line: line}
	go func() {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		resp, err := cl.Do(req)
		if err != nil {
			s := "unable to resolve"
			c.Message = s
			to <- c
			return
		}
		if resp.StatusCode != http.StatusOK {
			s := fmt.Sprintf("got status code %d", resp.StatusCode)
			c.Message = s
			to <- c
			return
		}
		c.Ok = true
		to <- c
	}()
	select {
	case res := <-to:
		ch <- res
	case <-time.After(5 * time.Second):
		s := fmt.Sprintf("%d: timeout after 5 seconds for %s", line, url)
		c.Message = s
		ch <- c

	}
}

func main() {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GIT_REPO_TOKEN")},
	)
	tc := oauth2.NewClient(ctx, ts)
	gh := github.NewClient(tc)
	cl := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	currDir, _ := os.Getwd()
	fName := fmt.Sprintf("%s/../%s", currDir, "docs-ecosystem/source/drivers/community-supported-drivers.txt")
	fmt.Println(fName)
	file, err := os.Open(fName)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	rxStrict := xurls.Strict()

	scanner := bufio.NewScanner(file)
	i := 1

	type Link struct {
		line int
		url  string
	}
	opq := make([]Link, 0)

	for scanner.Scan() {
		res := rxStrict.FindAllString(scanner.Text(), -1)
		if len(res) > 0 {
			for _, v := range res {
				opq = append(opq, Link{line: i, url: v})

			}
		}
		i++
	}

	var wg sync.WaitGroup
	ch := make(chan Check, len(opq))
	wg.Add(len(opq))

	for _, l := range opq {
		if strings.HasPrefix(l.url, TLSGIT) {
			go func(i int, v string, gh *github.Client) {
				info := SplitOrgAndRepo(v)
				CheckGitHub(i, info[0], info[1], ch, gh, &wg)
			}(l.line, l.url, gh)
		} else if strings.HasPrefix(l.url, GIT) {
			go func(i int, v string, gh *github.Client) {
				info := SplitOrgAndRepo(v)
				CheckGitHub(i, info[0], info[1], ch, gh, &wg)
			}(l.line, l.url, gh)
		} else {
			go func(i int, v string, cl *http.Client) {
				CheckLink(i, v, ch, cl, &wg)
			}(l.line, l.url, cl)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	wg.Wait()
	close(ch)
	for m := range ch {
		if !m.Ok {
			fmt.Println(fmt.Sprintf("%d: %s\n\t%s", m.Line, m.Source, m.Message))
		}
	}
}
