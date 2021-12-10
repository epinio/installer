package config

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// AddEnvToUsage adds env variables to help
func AddEnvToUsage(cmd *cobra.Command, argToEnv map[string]string) error {
	flagSet := make(map[string]bool)

	for arg, env := range argToEnv {
		if err := viper.BindEnv(arg, env); err != nil {
			return err
		}

		flag := cmd.Flag(arg)

		if flag != nil {
			flagSet[flag.Name] = true
			// add environment variable to the description
			flag.Usage = fmt.Sprintf("(%s) %s", env, flag.Usage)
		}
	}
	return nil
}
