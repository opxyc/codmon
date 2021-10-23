package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
)

// jsonConf defines the format in which gomon.json file should be parsed
type jsonConf struct {
	Watch   []string            `json:"watch"`
	Exclude map[string][]string `json:"exclude"`
	Cmd     string              `json:"cmd"`
}

// watcherConf is the configuration for watcher.
type watcherConf struct {
	ExcludedDirs  []string
	ExcludedFiles []string
	Pattern       *regexp.Regexp
}

// Returns the configuration for Watcher, commands to execute, attachStdin and verbose flags.
func get() (*watcherConf, *[]string, *bool, *bool) {
	// get returns:
	// (1) the config (files and folders to be excluded, file extensions to be watched for)
	// (2) the commands to be executed
	// (3) whether stdin is to be attached to the subprocesses(created for running the commands) and
	// (4) whether verbose output is needed
	// --
	// For (2) it checks the arguments, pipe and json file in the order of priority.
	watch, commands, attachStdin, verbose := parse()

	// prepare patten of file extensions to watch for
	// --
	// by default, watch all files
	pattern := regexp.MustCompile(`(.+\.*)$`)
	// use extensions provided via -w flag
	if len(*watch) > 0 {
		pattern = createPattern(watch)
		if *verbose {
			fmt.Printf("[gomon] watching for files: %+v\n", *watch)
		}
	}

	// Default configuration for Watcher
	var config = watcherConf{
		// Excludes no directories except hidden dirs
		ExcludedDirs: []string{},
		// Excludes no files except hidden files
		ExcludedFiles: []string{},
		// Watches all files
		Pattern: pattern,
	}

	// Get the configuration from json file
	jsonConf, err := getConfFromJSON()
	// If some error occured while reading the file, return the default cfg
	if err != nil {
		return &config, commands, attachStdin, verbose
	}

	if *verbose {
		fmt.Printf("[gomon] JSON file read: %+v\n", jsonConf)
	}

	// Frame a new pattern to watch for files
	// if -w flag is mentioned, it should override
	// file extensions mentioned via json configuraiton
	if len(*watch) == 0 && jsonConf.Watch != nil {
		config.Pattern = createPattern(&jsonConf.Watch)
		if *verbose {
			fmt.Printf("[gomon] watching for files: %+v\n", jsonConf.Watch)
		}
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

	// More priority is given to commands specified via argument and pipe.
	// So, only if flag is not given, try to get it from json file.
	if commands == nil && &jsonConf.Cmd != nil {
		var cmds []string
		for _, command := range strings.Split(jsonConf.Cmd, "&&") {
			trimmed := strings.Trim(command, " ")
			cmds = append(cmds, trimmed)
		}
		commands = &cmds
	}

	return &config, commands, attachStdin, verbose
}

// parse parses flags and inputs.
func parse() (watch *[]string, commands *[]string, stdin *bool, v *bool) {
	// commands - the commands to be excuted on file change
	// watch 	- the file extensions to watch for. ex: ["go", "c"]
	// stdin	- flag -stdin
	// v		- flag -v
	w := flag.String("w", "", fmt.Sprintf("file extensions to watch for\nEx: Use '%s -w go,c' to watch for .go and .c files", os.Args[0]))
	stdin = flag.Bool("stdin", false, "attach to stdin of executing commands")
	v = flag.Bool("v", false, "get verbose output (for debugging)")
	flag.Parse()

	var extnsToWatch []string
	if *w != "" {
		for _, ext := range strings.Split(*w, ",") {
			extnsToWatch = append(extnsToWatch, strings.Trim(ext, " "))
		}
	}

	// get the commands to be executed
	isPiped := false
	if flag.NArg() < 1 {
		// if arguments are not present, check for piped input
		stdinInf, err := os.Stdin.Stat()
		if err != nil || stdinInf.Mode()&os.ModeCharDevice != 0 {
			return &extnsToWatch, nil, stdin, v
		}

		isPiped = true
	}

	var cmdRaw string
	if !isPiped {
		cmdRaw = strings.Join(flag.Args()[:], " ")
	} else {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Printf("[!] could not read piped input: %v\n", err)
		}
		cmdRaw = string(b)
		// remove \n present at the end
		cmdRaw = cmdRaw[:len(cmdRaw)-1]
	}

	fmt.Println(cmdRaw)

	// commands are expected to be given in the format 'command1 && command2 ...'
	// so, split it with "&&" and form a slice
	var cmds []string
	for _, command := range strings.Split(cmdRaw, "&&") {
		cmds = append(cmds, strings.Trim(command, " "))
	}

	return &extnsToWatch, &cmds, stdin, v
}

func createPattern(extnsToWatch *[]string) *regexp.Regexp {
	pattern := ``
	if len(*extnsToWatch) != 0 {
		for _, fileExtension := range *extnsToWatch {
			pattern += fmt.Sprintf(`.+.%s$|`, fileExtension)
		}
	}

	return regexp.MustCompile(pattern[:len(pattern)-1])
}

func getPWD() string {
	pwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to get PWD:", err)
		os.Exit(2)
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
// Returns jsonConf if ok, error otherwise
func getConfFromJSON() (*jsonConf, error) {
	data, err := ioutil.ReadFile("./gomon.json")
	if err != nil {
		return nil, err
	}

	var config jsonConf
	err = json.Unmarshal(data, &config)
	if err != nil {
		fmt.Println("[!] Failed to parse gonom.json.")
		return nil, err
	}
	return &config, nil
}
