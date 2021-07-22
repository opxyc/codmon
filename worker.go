package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/fatih/color"
)

// hold the current subprocess details
var currentProcess *os.Process

// for logging stdout and stderr of subprocess
var pipeChan = make(chan io.ReadCloser)

// for writing gomon specific logs if -v flag is set
var loggerChan = make(chan string)

func worker(jobs <-chan string, commands []string, attachStdin *bool) {
	// watch for any interrupts/kill commands
	// if anything is received, kill the current running child process and exit
	go terminateCleanly()

	okToExecute := make(chan bool)
	go runCommands(commands, okToExecute, attachStdin)

	for {
		// receive a job
		<-jobs
		// kill current running process
		if currentProcess != nil {
			err := killProcess(currentProcess)
			if err != nil {
				fmt.Println(err)
			}
			currentProcess = nil
		}
		// inform that current process is killed
		okToExecute <- true
	}
}

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

func runCommands(commands []string, okToExecute <-chan bool, attachStdin *bool) {
	for {
		// wait for green signal
		// this indicates that the previously created process is killed
		// (so our program is not making orphaned processes)
		<-okToExecute

		// start running the commands
		go func() {
			for i, command := range commands {
				fmt.Printf(color.GreenString("[gomon · cmd #%d] %s\n", i+1, color.YellowString(command)))
				// start a new process
				cmd, stdoutPipe, stderrPipe, err := startCommand(command, attachStdin)
				if err != nil {
					fmt.Printf(color.RedString("Failed to start. Error: %v\n\n", err))
					continue
				}
				currentProcess = cmd.Process
				go writeResults(pipeChan)
				pipeChan <- stdoutPipe
				pipeChan <- stderrPipe

				// wait for it to finish
				_ = cmd.Wait()
			}
		}()
	}
}

// creates a child process and try to get it's stdout and stderr pipes
func startCommand(command string, attachStdin *bool) (cmd *exec.Cmd, stdout io.ReadCloser, stderr io.ReadCloser, err error) {
	args := strings.Split(command, " ")
	cmd = exec.Command(args[0], args[1:]...)
	if *attachStdin {
		cmd.Stdin = os.Stdin
	}

	if stdout, err = cmd.StdoutPipe(); err != nil {
		return
	}

	if stderr, err = cmd.StderrPipe(); err != nil {
		return
	}

	if err = cmd.Start(); err != nil {
		return
	}

	return
}

// WriteResults accepts a channel to which current running process sends it's
// stdout and stderr pipes
// It received the pipes and prints the data it received
func writeResults(pipeChan <-chan io.ReadCloser) {
	writer := func(pipe io.ReadCloser) {
		reader := bufio.NewReader(pipe)

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				continue
			}
			fmt.Printf("%s", line)
		}
	}

	for {
		pipe := <-pipeChan
		go writer(pipe)

		pipe = <-pipeChan
		go writer(pipe)
	}
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
