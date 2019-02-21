package pity

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/k0kubun/pp"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/taskie/osplus"
	"github.com/taskie/pity"
)

type Config struct {
	Input, Output, LogLevel string
}

var configFile string
var config Config
var (
	verbose, debug, version bool
)

const CommandName = "pity"

func init() {
	Command.PersistentFlags().StringVar(&configFile, "config", "", `config file (default: "pity.yml")`)
	Command.Flags().StringP("input", "i", "pity.txt", "pity input file")
	Command.Flags().StringP("output", "o", "", "terminal output file")
	Command.Flags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	Command.Flags().BoolVarP(&version, "version", "V", false, "show Version")
	Command.Flags().BoolVar(&debug, "debug", false, "debug output")

	viper.BindPFlag("Input", Command.Flags().Lookup("input"))
	viper.BindPFlag("Output", Command.Flags().Lookup("output"))

	cobra.OnInitialize(initConfig)
}

func initConfig() {
	if debug {
		log.SetLevel(log.DebugLevel)
	} else if verbose {
		log.SetLevel(log.InfoLevel)
	} else {
		log.SetLevel(log.WarnLevel)
	}

	if configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
		viper.SetConfigName(CommandName)
		conf, err := osplus.GetXdgConfigHome()
		if err != nil {
			log.Info(err)
		} else {
			viper.AddConfigPath(filepath.Join(conf, CommandName))
		}
		viper.AddConfigPath(".")
	}
	viper.SetEnvPrefix(CommandName)
	viper.AutomaticEnv()

	err := viper.ReadInConfig()
	if err != nil {
		log.Debug(err)
	}
	err = viper.Unmarshal(&config)
	if err != nil {
		log.Warn(err)
	}
}

func Main() {
	Command.Execute()
}

var Command = &cobra.Command{
	Use:  CommandName,
	Args: cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		err := run(cmd, args)
		if err != nil {
			log.Fatal(err)
		}
	},
}

func run(cmd *cobra.Command, args []string) error {
	if version {
		fmt.Println(pity.Version)
		return nil
	}
	if config.LogLevel != "" {
		lv, err := log.ParseLevel(config.LogLevel)
		if err != nil {
			log.Warn(err)
		} else {
			log.SetLevel(lv)
		}
	}
	if debug {
		if viper.ConfigFileUsed() != "" {
			log.Debugf("Using config file: %s", viper.ConfigFileUsed())
		}
		log.Debug(pp.Sprint(config))
	}

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
	inputFile, err := opener.Open(config.Input)
	if err != nil {
		return err
	}
	defer inputFile.Close()
	outputFile, err := opener.Create(config.Input)
	if err != nil {
		return err
	}
	defer outputFile.Close()

	return pity.NewExecutor(outputFile, inputFile, cmdName, cmdArgs...).Execute()
}
