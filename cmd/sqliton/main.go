package main

import (
	"embed"
	"github.com/spf13/cobra"
	"github.com/wesen/glazed/pkg/help"
	"github.com/wesen/sqliton/cmd/sqliton/cmds"
)

var rootCmd = &cobra.Command{
	Use:   "sqliton",
	Short: "sqliton runs SQL queries out of template files",
}

func main() {
	_ = rootCmd.Execute()
}

//go:embed doc/*
var docFS embed.FS

func init() {
	helpSystem := help.NewHelpSystem()
	err := helpSystem.LoadSectionsFromEmbedFS(docFS, ".")
	if err != nil {
		panic(err)
	}

	helpFunc, usageFunc := help.GetCobraHelpUsageFuncs(helpSystem)
	helpTemplate, usageTemplate := help.GetCobraHelpUsageTemplates(helpSystem)

	_ = usageFunc
	_ = usageTemplate

	rootCmd.SetHelpFunc(helpFunc)
	rootCmd.SetUsageFunc(usageFunc)
	rootCmd.SetHelpTemplate(helpTemplate)
	rootCmd.SetUsageTemplate(usageTemplate)

	helpCmd := help.NewCobraHelpCommand(helpSystem)
	rootCmd.SetHelpCommand(helpCmd)

	// db connection persistent base flags
	rootCmd.PersistentFlags().Bool("use-dbt-profiles", false, "Use dbt profiles.yml to connect to databases")
	rootCmd.PersistentFlags().String("dbt-profiles-path", "", "Path to dbt profiles.yml (default: ~/.dbt/profiles.yml)")
	rootCmd.PersistentFlags().String("dbt-profile", "default", "Name of dbt profile to use (default: default)")

	// more normal flags
	rootCmd.PersistentFlags().StringP("host", "H", "", "Database host")
	rootCmd.PersistentFlags().StringP("database", "D", "", "Database name")
	rootCmd.PersistentFlags().StringP("user", "u", "", "Database user")
	rootCmd.PersistentFlags().StringP("password", "p", "", "Database password")
	rootCmd.PersistentFlags().IntP("port", "P", 3306, "Database port")
	rootCmd.PersistentFlags().StringP("schema", "s", "", "Database schema (when applicable)")
	rootCmd.PersistentFlags().StringP("type", "t", "mysql", "Database type (mysql, postgres, etc.)")

	// dsn and driver
	rootCmd.PersistentFlags().String("dsn", "", "Database DSN")
	rootCmd.PersistentFlags().String("driver", "", "Database driver")

	rootCmd.AddCommand(cmds.DbCmd)
}
