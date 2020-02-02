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
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

// fileCmd represents the file command
var fileCmd = &cobra.Command{
	Use:   "file",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 1 {
			log.Fatal("file requires the path to the file to check")
		}
		ctx, cf := context.WithTimeout(context.Background(), 10*time.Second)
		defer cf()
		ck := checker.DefaultChecker(ctx)
		abs, _ := filepath.Abs(args[0])
		path := make(chan string, 1)
		path <- abs
		close(path)
		res, err := ck.CheckFiles(path)
		if err != nil {
			log.Fatal(err)
		}
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
	rootCmd.AddCommand(fileCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// fileCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// fileCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
