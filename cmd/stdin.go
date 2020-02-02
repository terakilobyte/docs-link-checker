/*
Copyright © 2020 Nathan Leniz <terakilobyte@gmail.com>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/

package cmd

import (
	"context"
	"docs-link-checker/checker"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// stdinCmd represents the stdin command
var stdinCmd = &cobra.Command{
	Use:   "stdin",
	Short: "Runs the link checker on input from stdin",
	Long: `Pipe stdin to the application with xargs, or
provide a url to check.

docs-link-checker stdin https://www.google.com
docs-link-checker stdin "$(cat links.txt)" -o linksTest.json

`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 1 {
			log.Fatal("stdin requires one argument")
		}
		ctx, cf := context.WithTimeout(context.Background(), 10*time.Second)
		defer cf()
		ck := checker.DefaultChecker(ctx)
		res := make(chan *checker.Check, 1000)
		ck.PerformURLChecks(strings.NewReader(args[0]), "stdin", res)
		output := make(checker.Result)
		for m := range res {
			if v, ok := output[m.File]; !ok {
				outer := checker.ResultOuter{Line: make(map[int]checker.ResultInner)}
				outer.Line[m.Line] = checker.ResultInner{URL: m.URL, Message: m.Message}
				output[m.File] = outer
			} else {
				v.Line[m.Line] = checker.ResultInner{URL: m.URL, Message: m.Message}
			}
		}
		jsonString, _ := json.MarshalIndent(output, "", "  ")
		if outfile != "" {
			f, err := os.Create(outfile)
			if err != nil {
				log.Fatal(err)
			}
			defer f.Close()
			fmt.Println("writing output to", outfile)
			f.Write(jsonString)
		} else {
			fmt.Println(string(jsonString))
		}
	},
}

func init() {
	rootCmd.AddCommand(stdinCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// stdinCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// stdinCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
