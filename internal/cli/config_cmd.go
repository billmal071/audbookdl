package cli

import (
	"fmt"

	"github.com/billmal071/audbookdl/internal/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
}

var configGetCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Get a configuration value",
	Args:  cobra.ExactArgs(1),
	RunE:  runConfigGet,
}

var configSetCmd = &cobra.Command{
	Use:   "set [key] [value]",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	RunE:  runConfigSet,
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show the config file path",
	Run:   runConfigPath,
}

func init() {
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configPathCmd)
}

func runConfigGet(cmd *cobra.Command, args []string) error {
	key := args[0]
	value := config.GetValue(key)
	if value == nil {
		fmt.Printf("%s = (not set)\n", key)
		return nil
	}
	fmt.Printf("%s = %v\n", key, value)
	return nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key, value := args[0], args[1]
	if err := config.Set(key, value); err != nil {
		return fmt.Errorf("set config %q: %w", key, err)
	}
	fmt.Printf("Set %s = %s\n", key, value)
	return nil
}

func runConfigPath(cmd *cobra.Command, args []string) {
	fmt.Println(config.GetConfigPath())
}
