/*
Copyright © 2020 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/verifa/bubbly/config"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

var (
	globalServerConfig config.ServerConfig
	globalConfigFile   string
	// globalTest         string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "bubbly",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		// fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	f := rootCmd.PersistentFlags()

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	f.StringVar(&globalConfigFile, "config", "", "config file (default is $HOME/.bubbly.yaml)")
	// viper.BindPFlag("globalConfigFile", rootCmd.Flags().Lookup("config"))

	// f.StringVar(&globalTest, "test", "", "A test flag")
	f.StringVar(&globalServerConfig.Host, "host", "", "bubbly server host")
	f.StringVar(&globalServerConfig.Port, "port", "", "bubbly server port")
	f.BoolVar(&globalServerConfig.Auth, "auth", false, "bubbly server auth")
	f.StringVar(&globalServerConfig.Token, "token", "", "bubbly server token")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	viper.BindPFlags(f)
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if globalConfigFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(globalConfigFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".bubbly" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".bubbly")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		for _, v := range viper.AllKeys() {
			fmt.Println(v, viper.Get(v))
		}
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
