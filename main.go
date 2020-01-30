package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
	"golang.org/x/sync/errgroup"
	"mvdan.cc/xurls/v2"
)

type Check struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Source  string `json:"source"`
	Message string `json:"message"`
	Ok      bool   `json:"ok"`
}

type Link struct {
	line int
	url  string
	file string
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

func CheckGitHub(l Link, org, repo string, ch chan<- Check, gh *github.Client, wg *sync.WaitGroup) {
	to := make(chan Check, 1)
	defer close(to)
	defer wg.Done()
	c := Check{Source: fmt.Sprintf("%s/%s", org, repo), Line: l.line, File: l.file}
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

func CheckLink(l Link, ch chan<- Check, cl *http.Client, wg *sync.WaitGroup) {
	to := make(chan Check, 1)
	defer close(to)
	defer wg.Done()
	c := Check{Source: l.url, Line: l.line, File: l.file}
	go func() {
		req, err := http.NewRequest(http.MethodGet, l.url, nil)
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
		s := "timeout after 5 seconds"
		c.Message = s
		ch <- c

	}
}

func main() {

	currDir, _ := os.Getwd()

	ctx, cf := context.WithTimeout(context.Background(), 5*time.Second)
	defer cf()
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
	paths, err := search(ctx, currDir)
	if err != nil {
		log.Fatal(err)
	}
	res := checkUrls(ctx, paths, gh, cl)
	for c := range res {
		fmt.Println(c.File, c.Line, c.Ok)
	}

}

func checkUrls(ctx context.Context, paths chan string, gh *github.Client, cl *http.Client) <-chan Check {
	g, ctx := errgroup.WithContext(ctx)
	rxStrict := xurls.Strict()
	c := make(chan string, 1000)
	for path := range paths {
		p := path
		g.Go(func() error {

			select {
			case c <- p:
			case <-ctx.Done():
				return ctx.Err()
			}
			return nil
		})
	}
	go func() {
		g.Wait()
		close(c)
	}()

	checks := make(chan Check, 1000)

	for file := range c {
		fName := file
		g.Go(func() error {
			var wg sync.WaitGroup
			var ch chan Check
			file, err := os.Open(fName)
			if err != nil {
				log.Fatal(err)
			}
			defer file.Close()

			scanner := bufio.NewScanner(file)
			i := 1

			opq := make([]Link, 0)

			for scanner.Scan() {
				res := rxStrict.FindAllString(scanner.Text(), -1)
				if len(res) > 0 {
					for _, v := range res {
						opq = append(opq, Link{line: i, url: v, file: fName})

					}
				}
				i++
			}

			ch = make(chan Check, len(opq))
			wg.Add(len(opq))

			for _, l := range opq {
				if strings.HasPrefix(l.url, TLSGIT) {
					go func(l Link) {
						info := SplitOrgAndRepo(l.url)
						CheckGitHub(l, info[0], info[1], ch, gh, &wg)
					}(l)
				} else if strings.HasPrefix(l.url, GIT) {
					go func(l Link) {
						info := SplitOrgAndRepo(l.url)
						CheckGitHub(l, info[0], info[1], ch, gh, &wg)
					}(l)
				} else {
					go func(l Link) {
						CheckLink(l, ch, cl, &wg)
					}(l)
				}
			}

			if err := scanner.Err(); err != nil {
				log.Fatal(err)
			}
			wg.Wait()
			close(ch)
			for m := range ch {
				checks <- m
			}
			return nil
		})
	}
	return checks
}

func search(ctx context.Context, root string) (chan string, error) {
	g, ctx := errgroup.WithContext(ctx)
	paths := make(chan string, 1000)
	// get all the paths

	g.Go(func() error {
		defer close(paths)

		return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return nil
			}
			if !info.IsDir() && (!strings.HasSuffix(info.Name(), ".rst") || !strings.HasSuffix(info.Name(), ".txt")) {
				return nil
			}

			select {
			case paths <- path:
			case <-ctx.Done():
				return ctx.Err()
			}
			return nil
		})
	})

	return paths, g.Wait()

}
