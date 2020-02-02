package checker

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/github"
	"github.com/spf13/viper"
	"golang.org/x/oauth2"
	"golang.org/x/sync/errgroup"
	"mvdan.cc/xurls/v2"
)

// Check holds all the information about a URL/Github check
type Check struct {
	Line    int    `json:"line"`
	URL     string `json:"url"`
	Message string `json:"message"`
	Ok      bool   `json:"ok"`
	File    string `json:"file"`
}

// Result is the top level result type. Each entry is a file name
type Result map[string]ResultOuter

// ResultOuter is the outer result type. Each entry is a line number in a file
type ResultOuter struct {
	Line map[int]ResultInner `json:"line warnings"`
}

// ResultInner holds the information that is assigned to a line number in the
// overall result
type ResultInner struct {
	URL     string `json:"url"`
	Message string `json:"message"`
}

const (
	githttps = "https://github.com/"
	githttp  = "http://github.com/"
)

// SplitOrgAndRepo strips the 'https://github.com/' or 'http://github.com' from
// a url, then splits that url into the org and repo
func SplitOrgAndRepo(url string) (string, string, error) {
	if strings.HasPrefix(url, githttps) {
		res := strings.Split(strings.TrimPrefix(url, githttps), "/")
		if len(res) == 2 {
			return res[0], res[1], nil
		}
	}
	res := strings.Split(strings.TrimPrefix(url, githttp), "/")
	if len(res) == 2 {
		return res[0], res[1], nil
	}
	return "", "", errors.New("Invalid split operation")
}

// CheckGithub checks a Github org for status and last updated
// Reports back if the repo isn't a 200 status OR if the repo hasn't had any
// activity within the last year
func (c *Check) CheckGithub(ch chan<- *Check, gh *github.Client) {
	to := make(chan *Check, 1)
	org, repo, err := SplitOrgAndRepo(c.URL)
	if err != nil {

	}
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

// CheckLink checks to make sure a URL returns a 200
func (c *Check) CheckLink(ch chan<- *Check, cl *http.Client) {
	to := make(chan *Check, 1)
	go func() {
		req, err := http.NewRequest(http.MethodGet, c.URL, nil)
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

type Checker struct {
	ctx context.Context
	g   *errgroup.Group
	gh  *github.Client
	cl  *http.Client
}

func NewChecker(ctx context.Context, g *errgroup.Group, gh *github.Client, cl *http.Client) Checker {
	return Checker{ctx, g, gh, cl}
}

func DefaultChecker(ctx context.Context) Checker {
	g, ctx := errgroup.WithContext(ctx)
	gt := viper.Get("GIT_REPO_TOKEN")
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: gt.(string)},
	)
	tc := oauth2.NewClient(ctx, ts)
	gh := github.NewClient(tc)
	cl := &http.Client{
		// Overriding default CheckRedirect so that redirects are reported
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return Checker{ctx, g, gh, cl}
}

// FindCandidateFiles finds all .rst or .txt files in the current directory and
// its children
func (ck *Checker) FindCandidateFiles(dir string) (chan string, error) {
	paths := make(chan string, 10000)
	// get all the paths
	ck.g.Go(func() error {
		defer close(paths)
		return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return nil
			}

			if strings.HasSuffix(info.Name(), ".rst") || strings.HasSuffix(info.Name(), ".txt") {
				select {
				case paths <- path:
				case <-ck.ctx.Done():
					return ck.ctx.Err()
				}
			}
			return nil
		})

	})
	return paths, ck.g.Wait()
}

func (ck *Checker) CheckFiles(paths chan string) (chan *Check, error) {
	res := make(chan *Check, 1000)
	for f := range paths {
		f := f
		ck.g.Go(func() error {
			file, err := os.Open(f)
			if err != nil {
				log.Fatal(err)
			}
			defer file.Close()
			ck.PerformURLChecks(file, f, res)
			return nil
		})
	}
	fmt.Println(len(res))
	return res, ck.g.Wait()
}

func (ck *Checker) PerformURLChecks(r io.Reader, f string, res chan *Check) {
	defer close(res)
	rxStrict := xurls.Strict()
	scanner := bufio.NewScanner(r)
	lineNum := 1
	opq := make([]Check, 0)
	for scanner.Scan() {
		res := rxStrict.FindAllString(scanner.Text(), -1)
		if len(res) > 0 {
			for _, url := range res {
				opq = append(opq, Check{File: filepath.Base(f), URL: url, Line: lineNum})
			}
		}
		lineNum++
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Add(len(opq))
	ch := make(chan *Check, len(opq))
	for _, c := range opq {
		c := c
		if strings.HasPrefix(c.URL, githttps) || strings.HasPrefix(c.URL, githttp) {
			go func() {
				defer wg.Done()
				c.CheckGithub(ch, ck.gh)
			}()
		} else {
			go func() {
				defer wg.Done()
				c.CheckLink(ch, ck.cl)
			}()
		}
	}

	wg.Wait()
	close(ch)
	for m := range ch {
		if !m.Ok {
			res <- m
		}
	}
}
