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
	Source string `json:"source"`
	Result string `json:"result"`
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

func CheckGitHub(line int, org, repo string, ch chan<- string, gh *github.Client, wg *sync.WaitGroup) {
	to := make(chan string, 1)
	go func() {
		defer wg.Done()
		stats, resp, err := gh.Repositories.Get(context.Background(), org, repo)
		if err != nil {
			s := fmt.Sprintf("%d: just some weirdness with %s/%s, err: %q", line, org, repo, err)
			fmt.Println(s)
			to <- s
			return
		}
		if resp.StatusCode == http.StatusOK {
			if stats.PushedAt.AddDate(1, 0, 0).Before(time.Now()) {
				s := fmt.Sprintf("%d: %s/%s is stale (last commit more than a year ago)", line, org, repo)
				fmt.Println(s)
				to <- s
				return

			}
			s := fmt.Sprintf("%d: %s/%s ok", line, org, repo)
			fmt.Println(s)
			to <- s
		} else {
			s := fmt.Sprintf("%d: got %d status for %s/%s", line, resp.StatusCode, org, repo)
			fmt.Println(s)
			to <- s
		}
	}()
	select {
	case res := <-to:
		ch <- res
	case <-time.After(5 * time.Second):
		s := fmt.Sprintf("%d: timeout after 5 seconds for %s/%s", line, org, repo)
		fmt.Println(s)
		ch <- s
	}
}

func CheckLink(line int, url string, ch chan<- string, cl *http.Client, wg *sync.WaitGroup) {
	to := make(chan string, 1)
	go func() {
		defer wg.Done()
		req, err := http.NewRequest(http.MethodGet, url, nil)
		resp, err := cl.Do(req)
		if err != nil {
			s := fmt.Sprintf("%d: unable to resolve %s", line, url)
			fmt.Println(s)
			to <- s
			return
		}
		if resp.StatusCode != http.StatusOK {
			s := fmt.Sprintf("%d: got status code %d for %s", line, resp.StatusCode, url)
			fmt.Println(s)
			to <- s
			return
		}
		s := fmt.Sprintf("%d: %s returned %s", line, url, resp.Status)
		fmt.Println(s)
		to <- s
	}()
	select {
	case res := <-to:
		ch <- res
	case <-time.After(5 * time.Second):
		s := fmt.Sprintf("%d: timeout after 5 seconds for %s", line, url)
		fmt.Println(s)
		ch <- s

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
	ch := make(chan string, len(opq))
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
		fmt.Sprintf("%s", m)
	}
}
