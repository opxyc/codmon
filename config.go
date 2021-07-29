package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
)

// JSONConfig - defines the format in which gomon.json file should be parsed
type JSONConfig struct {
	Watch   []string            `json:"watch"`
	Exclude map[string][]string `json:"exclude"`
	Cmd     string              `json:"cmd"`
}

type WatcherConfig struct {
	ExcludedDirs  []string
	ExcludedFiles []string
	Pattern       *regexp.Regexp
}

// GetConf returns:
// (1) the config (files and folders to be excluded, file extensions to be watched for)
// (2) the commands to be executed and
// (3) whether stdin is to be attached to the subprocesses(created for running the commands)

// Returns the default config, if no gomon.json file is available
// for (2) it checks both json file and the flags
// for (3) it checks flag only.
func getConfig() (*WatcherConfig, *[]string, *bool, *bool) {
	// Get commands and attach stdin from flags
	commands, attachStdin, verbose := parseFlags()

	// Default configuration for Watcher
	var config = WatcherConfig{
		// Excludes no directories except hidden dirs
		ExcludedDirs: []string{},
		// Excludes no files except hidden files
		ExcludedFiles: []string{},
		// Watches all .go files
		Pattern: regexp.MustCompile(`(.+\.*)$`),
	}

	// Get the configuration from json file
	jsonConf, err := getConfFromJSON()
	// If some error occured while reading the file, return the default cfg
	if err != nil {
		return &config, commands, attachStdin, verbose
	}

	// Frame a new pattern to watch for files
	if jsonConf.Watch != nil {
		pattern := ``
		if len(jsonConf.Watch) != 0 {
			for _, fileExtension := range jsonConf.Watch {
				pattern += fmt.Sprintf(`.+.%s$|`, fileExtension)
			}
		}
		config.Pattern = regexp.MustCompile(pattern[:len(pattern)-1])
	}

	// get Persent Working Directory
	pwd := getPWD()

	// Format the pattern specified for excluded directories
	for _, pattern := range jsonConf.Exclude["dirs"] {
		formatDirPattern(&pattern, &pwd)
		config.ExcludedDirs = append(config.ExcludedDirs, pattern)
	}

	// Format the pattern specified for excluded files
	for _, pattern := range jsonConf.Exclude["files"] {
		formatFilePattern(&pattern, &pwd)
		config.ExcludedFiles = append(config.ExcludedFiles, pattern)
	}

	// More priority is given to "cmd" mentioned in flag
	// So, only if flag is not given, try to get it from json file.
	if commands == nil && &jsonConf.Cmd != nil && strings.Trim(jsonConf.Cmd, " ") != "" {
		var cmds []string
		for _, command := range strings.Split(jsonConf.Cmd, "&&") {
			trimmed := strings.Trim(command, " ")
			cmds = append(cmds, trimmed)
		}
		commands = &cmds
	}

	return &config, commands, attachStdin, verbose
}

// parses the flags
// Output:
// return &commands, flagStdin
// 		if no --cmd is mentioned   => commands => nil
// 		if no --stdin is mentioned => flagStdin = false
func parseFlags() (*[]string, *bool, *bool) {
	flagCmd := flag.String("cmd", "", "Specifies the commands to be execute. 'command1 [&& command2 ...]'")
	flagStdin := flag.Bool("stdin", false, "If specified, will attach to stdin of subprocess")
	flagVerbose := flag.Bool("v", false, "If specified, gomon specific logs will be printed")
	flag.Parse()

	// if "cmd" is not mentioned, return nil
	if strings.Trim(*flagCmd, " ") == "" {
		return nil, flagStdin, flagVerbose
	}

	// "cmd" is expected to be in the format 'command1 && command2 ...'
	// so, split it with "&&" and form a slice
	var commands []string
	for _, command := range strings.Split(*flagCmd, "&&") {
		trimmed := strings.Trim(command, " ")
		commands = append(commands, trimmed)
	}

	return &commands, flagStdin, flagVerbose
}

func getPWD() string {
	pwd, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	return strings.Replace(pwd, "\\", "/", len(pwd))
}

func formatDirPattern(pattern *string, pwd *string) {
	if len(*pattern) > 0 {
		*pattern = *pwd + "/" + *pattern
		*pattern = strings.ReplaceAll(*pattern, "*", "(.*)")
		if startingWithAlphabet, _ := regexp.MatchString(`^[a-zA-Z]{1}`, *pattern); startingWithAlphabet {
			*pattern = fmt.Sprintf("^%s", *pattern)
		}
		*pattern = fmt.Sprintf("%s(.*)$", *pattern)
	}
}

func formatFilePattern(pattern *string, pwd *string) {
	if len(*pattern) > 0 {
		*pattern = strings.ReplaceAll(*pattern, "*", "(.*)")
		if endingWithAlphabet, _ := regexp.MatchString(`(.*)[a-zA-Z]$`, *pattern); endingWithAlphabet {
			*pattern = fmt.Sprintf("%s$", *pattern)
		}
		*pattern = fmt.Sprintf(`^%s`, *pattern)
	}
}

// getConfFromJSON simply tries to read the json file.
// Returns JSONConfig if ok, error otherwise
func getConfFromJSON() (*JSONConfig, error) {
	data, err := ioutil.ReadFile("./gomon.json")
	if err != nil {
		return nil, err
	}

	var config JSONConfig
	err = json.Unmarshal(data, &config)
	if err != nil {
		fmt.Println("[!] Failed to parse gonom.json.")
		return nil, err
	}
	return &config, nil
}
