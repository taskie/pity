package pity

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/taskie/osplus"
	"github.com/taskie/pity"
)

var (
	cfgFile, input, output string
)

func init() {
	cobra.OnInitialize(initConfig)
	Command.PersistentFlags().StringVar(&cfgFile, "config", "c", "config file (default is $XDG_CONFIG_HOME/pity/pity.yaml)")
	Command.Flags().StringVarP(&input, "input", "i", "pity.txt", "pity input file")
	Command.Flags().StringVarP(&output, "output", "o", "", "terminal output file")
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		conf, err := osplus.GetXdgConfigHome()
		if err != nil {
			panic(err)
		}
		viper.AddConfigPath(filepath.Join(conf, "pity"))
		viper.SetConfigName("pity")
	}
	viper.AutomaticEnv()
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}

func Main() {
	Command.Execute()
}

var Command = &cobra.Command{
	Use: "pity",
	Run: func(cmd *cobra.Command, args []string) {
		err := run(cmd, args)
		if err != nil {
			logrus.Fatal(err)
		}
	},
}

func run(cmd *cobra.Command, args []string) error {
	cmdName := os.Getenv("SHELL")
	if cmdName == "" {
		cmdName = "sh"
	}
	cmdArgs := []string{}
	if len(args) != 0 {
		cmdName = args[0]
		cmdArgs = args[1:]
	}

	opener := osplus.NewOpener()
	opener.Unbuffered = true
	inputFile, err := opener.Open(input)
	if err != nil {
		return err
	}
	outputFile, err := opener.Create(output)
	if err != nil {
		return err
	}

	return pity.NewExecutor(outputFile, inputFile, cmdName, cmdArgs...).Execute()
}
