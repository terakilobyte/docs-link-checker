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
	"fmt"
	"os"

	"github.com/spf13/cobra"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

var cfgFile string
var outfile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "docs-link-checker",
	Short: "docs-link-checker checks all encountered http and https urls and reports non 200 status",
	Long: `Checks all http and https urls and reports if they return a non 200 code.
Additionally, checks github repos encountered and will report if they are stale, i.e.
if the last acitivity is more than a year ago.

docs-link-checker dir ~/mongo/docs-ecosystem --outfile results.json
docs-link-checker file ~/mongo/docs-ecosystem/community-supported-drivers.txt

If -o (--out) is specified, will attemp to write to the specified file.

Supports directory search and checking with the dir sub-command.
Supports individual file checking with the file sub-command.

docs-link-checker requires a Github API key to avoid being rate limited when it
is checking for repository freshness. You can generate a new API key by visiting:
https://github.com/settings/tokens

The token only needs the repo permission group.

Store the token either in your environment, or in a YAML config file. By default,
the program will check $HOME/.docs-link-checker.yaml
`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) {},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.docs-link-checker.yaml)")
	rootCmd.PersistentFlags().StringVarP(&outfile, "out", "o", "", "-o output.json")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".docs-link-checker" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".docs-link-checker")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
