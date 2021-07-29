package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
)

// hold the current subprocess details
var currentProcess *os.Process

// for logging stdout and stderr of subprocess
var pipeChan = make(chan io.ReadCloser)

func worker(jobs <-chan string, commands []string, attachStdin *bool, verbose *bool) {
	// watch for any interrupts/kill commands
	// if anything is received, kill the current running child process and exit
	go terminateCleanly()

	okToExecute := make(chan bool)
	go runCommands(commands, okToExecute, attachStdin, verbose)

	for {
		// receive a job
		<-jobs
		// kill current running process
		if currentProcess != nil {
			pid := currentProcess.Pid
			err := killProcess(currentProcess)
			if err != nil && *verbose {
				fmt.Printf("[gomon] Failed to kill process %d. Error: %v\n", pid, err)
			} else if err == nil && *verbose {
				fmt.Printf("[gomon] Killed process %d\n", pid)
			}
		}
		// give a little pause so that if any process it killed,
		// it's status is logged to console
		time.Sleep(time.Millisecond * 1000)
		// inform that current process is killed
		okToExecute <- true
	}
}

func runCommands(commands []string, okToExecute <-chan bool, attachStdin *bool, verbose *bool) {
	for {
		// wait for green signal
		// this indicates that the previously created process is killed
		// (so our program is not making orphaned processes)
		<-okToExecute

		// start running the commands
		go func() {
			for _, command := range commands {
				fmt.Println(color.CyanString("> %s", command))
				// start a new process
				cmd, err := startCommand(command, attachStdin)
				if err != nil {
					fmt.Printf(color.RedString("[gomon] Failed to start. Error: %v\n", err))
					continue
				}

				currentProcess = cmd.Process
				// go writeResults(pipeChan)
				if *verbose {
					fmt.Printf("[gomon] Process %d created for executing '%s'\n", currentProcess.Pid, command)
				}

				// wait for it to finish
				err = cmd.Wait()
				if *verbose {
					if err != nil {
						if currentProcess != nil {
							fmt.Printf("[gomon] Process %d terminated with error or was killed. Error: %v\n", cmd.Process.Pid, err)
							currentProcess = nil
						}
					} else {
						fmt.Printf("[gomon] Process %d completed successfully\n", cmd.Process.Pid)
					}
				}
			}
			return
		}()
	}
}

// creates a child process and try to get it's stdout and stderr pipes
func startCommand(command string, attachStdin *bool) (*exec.Cmd, error) {
	args := strings.Split(command, " ")
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if *attachStdin {
		cmd.Stdin = os.Stdin
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return cmd, nil
}

// killProcess kills a process(hard kill)
func killProcess(process *os.Process) error {
	err := process.Kill()
	if err != nil {
		return err
	}
	currentProcess = nil
	return nil
}

// terminateCleanly listens for interrupts and try to kill currently
// running subprocess before exiting.
func terminateCleanly() {
	var fatalSignals = []os.Signal{
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, fatalSignals...)
	<-signalChan

	fmt.Println(color.CyanString("Exiting.."))
	if currentProcess != nil {
		killProcess(currentProcess)
	}
	os.Exit(0)
}
