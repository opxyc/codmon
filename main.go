package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/radovskyb/watcher"
)

func main() {
	// Get config from json and command line flags
	config, commands, attachStdin, verbose := getConfig()
	// if no commands are mentioned for execution after file modification, then
	// gomon has nothing to do. Simply exit.
	if commands == nil {
		fmt.Println("Usage: \ngomon -cmd 'command1 [&& command]' [-stdin]")
		fmt.Println("      -cmd     Specify the commands to be executed after file change has occured")
		fmt.Println("      -stdin   Attach to STDIN if the supprocesses")
		fmt.Println("               (which are created for running the commands)")
		return
	}

	// "job" entering the jobs channel is consumed by the "worker".
	// What is a "job" and what does "worker" do?
	// On every file update, the path of that file(which is returned by the Watcher) is sent to jobs channel.
	// Upon receiving such a messsage, the worker will start executing the commands mentioned
	// either in --cmd or in the json file.
	// Before directly executing the command, the worker also makes sures to kill any process
	// it has started before (when it received the previous file change message)
	jobs := make(chan string)

	// watcher comes from "github.com/radovskyb/watcher" package
	w := watcher.New()
	defer w.Close()
	w.SetMaxEvents(1)

	// Only notify rename, move, create and update events.
	w.FilterOps(watcher.Rename, watcher.Move, watcher.Create, watcher.Write)
	// Watch files that matches the given pattern
	w.AddFilterHook(watcher.RegexFilterHook(config.Pattern, false))

	// Watch this folder for changes.
	if err := w.AddRecursive("."); err != nil {
		fmt.Println(err)
		return
	}

	// watch for file changes
	go watch(w, config, jobs)
	// run the commands on file change
	go worker(jobs, *commands, attachStdin, verbose)
	// to run the commands on startup, send a message to the channel
	// on receiving message, worker will start it's job
	jobs <- "start"

	// Start the watching process
	if err := w.Start(time.Millisecond * 500); err != nil {
		fmt.Println(err)
		return
	}
}

// watch watches for file changes
// when it detects any change, it will sent a message to jobs channel
func watch(w *watcher.Watcher, config *WatcherConfig, jobs chan<- string) {
	// wait for send message to jobs channel since sometimes,
	// user may press save multiple times so quickly
	// which will make worker do unnecessary execution of commands
	var prevMsgSent, currentTime time.Time
	for {
		select {
		case event := <-w.Event:
			if isItWorthIt(event.Path, config) {
				currentTime = time.Now()
				// if time difference < 1sec, dont bother
				if !(currentTime.Sub(prevMsgSent) < time.Second*3) {
					jobs <- event.Path
					prevMsgSent = currentTime
				}
			}
		case err := <-w.Error:
			fmt.Println(err)
			return
		case <-w.Closed:
			return
		}
	}
}

// isItWorthIt checks if the file change which is detected by the watcher
// is worth running all the commands mentioned once again.
// How does it decide the worth?
// * if the directory in which the change occured is mentioned in "exclude.dirs", then it's not worthy
// * if the file which was changed in mentioned in the "exclude.files", then it's not worthy
func isItWorthIt(filePath string, config *WatcherConfig) bool {
	dir := filepath.Dir(filePath)
	dir = strings.Replace(dir, "\\", "/", 99)
	for _, d := range config.ExcludedDirs {
		if matched, _ := regexp.MatchString(d, dir); matched {
			return false
		}
	}

	base := filepath.Base(filePath)
	for _, f := range config.ExcludedFiles {
		if matched, _ := regexp.MatchString(f, base); matched {
			return false
		}
	}

	return true
}
