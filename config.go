package main

import (
	flag "github.com/spf13/pflag"			// Commandline flags
	viper "github.com/spf13/viper"			// Configuration and env-var configuration
	"fmt"						// formated I/O
	_ "strconv"					// Convert strings to []byte etc
	_ "net/url"					// URL parsing
	_ "os"						// Environment variables and exit
	_ "time"					// Timeouts and sleep
	_ "encoding/json"				// JSON parsing
	_ "context"
	_ "io/ioutil"					// Reading files
//	"database/sql"
//	"github.com/go-ble/ble/linux/adv"
//	"encoding/hex"
//	_ "github.com/denisenkom/go-mssqldb"
)

// See process_configuration for how these configuration variables get set:
var (
	debugEnabled			bool
	sqlite_file			string
	file_roots			[]string
)

func process_configuration() {
	/*
	 * Prepare Commandline flags
	 */
	flag.String("db", "", "SQLite3 database filename")
	flag.Bool("debug", false, "enable debug")
	flag.StringArray("root", file_roots, "Filesystem roots")
 	flag.Parse()

	/*
	 * Make pflags available in viper
	 */
	viper.BindPFlags(flag.CommandLine);			// All, no renaming
	viper.BindPFlag("database", flag.Lookup("db"));// cmdline is --db, config var is database

	/*
	 * Prepare environment variables for viper variables
	 */
	viper.SetEnvPrefix("FSF");	// E.g. FSF_DATABASE
	viper.AutomaticEnv();		// Grab any FSF_XXX env var for Get("xxx")
	// viper.BindEnv("some_key", "SOME_ENV_VAR");		// Specific binding

	/*
	 * Load the config file
	 */
	viper.SetConfigType("json")
	viper.SetConfigName("fsfingerprint.json");
	viper.AddConfigPath(".");
	viper.AddConfigPath("$HOME");
	err := viper.ReadInConfig();
	_, ok := err.(viper.ConfigFileNotFoundError);
	if err == nil {
		fmt.Printf("using config %s\n", viper.ConfigFileUsed());
	} else if ok {
		// fmt.Printf("No config found\n");
	} else {
		// Maybe a syntax error?
		panic(fmt.Errorf("config file error: %w", err));
	}

	/*
	 * Fetch the variables
	 */
	debugEnabled = viper.GetBool("debug");
	file_roots = viper.GetStringSlice("root");

	sqlite_file = viper.GetString("db");
}
