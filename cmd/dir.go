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
	"time"

	"github.com/spf13/cobra"
)

// dirCmd represents the dir command
var dirCmd = &cobra.Command{
	Use:   "dir <path>",
	Short: "Runs the link checker in the specified directory.",
	Long: `Runs the link checker, finding all .rst and .txt files in the specified directory,
and all child directories. It will not look in directories that begin with a period (.)

If no directory is specified, it uses the directory the program is invoked from.`,

	Run: func(cmd *cobra.Command, args []string) {
		ctx, cf := context.WithTimeout(context.Background(), 10*time.Second)
		defer cf()
		var dir string
		if len(args) < 1 || args[0] == "." {
			dir, _ = os.Getwd()
		} else {
			dir = args[0]
		}
		ck := checker.DefaultChecker(ctx)
		paths, err := ck.FindCandidateFiles(dir)
		if err != nil {
			log.Fatal(err)
		}
		res, err := ck.CheckFiles(paths)

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
		jsonString, err := json.MarshalIndent(output, "", "  ")
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
	rootCmd.AddCommand(dirCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// dirCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// dirCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
