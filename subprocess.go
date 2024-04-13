package main

import (
	"bufio"
	"github.com/seculize/islazy/log"
	"os"
	"os/exec"
	"path"
	"strings"
)

func getKubeVersion() string {
	version := startProcess("kubectl.exe", "version")
	return version
}
func startProcess(processPath string, arguments ...string) string {
	workingDir, _ := os.Getwd()
	processPath = path.Join(workingDir, processPath)
	cmd := exec.Command(processPath, arguments...)
	args := ""
	for _, arg := range arguments {
		args += arg + " "
	}
	log.Debug("Running command: %s %s", processPath, args)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+InstallerArgs.Kubeconfig)
	cmdReader, err := cmd.StdoutPipe()
	cmdErrorReader, err := cmd.StderrPipe()
	output := ""
	if err != nil {
		log.Fatal("Error creating StdoutPipe for Cmd", err)
	}
	scanner := bufio.NewScanner(cmdReader)
	done := make(chan bool)
	go func() {
		for scanner.Scan() {
			output += scanner.Text() + "\n"
			log.Info(scanner.Text())
		}
		done <- true
	}()
	scannerError := bufio.NewScanner(cmdErrorReader)
	doneErr := make(chan bool)
	go func() {
		for scannerError.Scan() {
			output += scannerError.Text() + "\n"
			log.Error(scannerError.Text())
		}
		doneErr <- true
	}()
	err = cmd.Start()
	if err != nil {
		log.Warning("Error starting Cmd", err.Error())
		return ""
	}
	<-done
	<-doneErr
	err = cmd.Wait()
	return output
}
func startProcessWithStdIn(processPath string, content string, arguments ...string) string {
	workingDir, _ := os.Getwd()
	processPath = path.Join(workingDir, processPath)
	cmd := exec.Command(processPath, arguments...)
	args := ""
	for _, arg := range arguments {
		args += arg + " "
	}
	log.Debug("Running command: %s %s", processPath, args)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+InstallerArgs.Kubeconfig)
	cmdReader, err := cmd.StdoutPipe()
	cmdErrorReader, err := cmd.StderrPipe()
	output := ""
	if err != nil {
		log.Fatal("Error creating StdoutPipe for Cmd", err)
	}
	scanner := bufio.NewScanner(cmdReader)
	done := make(chan bool)
	go func() {
		for scanner.Scan() {
			output += scanner.Text() + "\n"
			log.Info(scanner.Text())
		}
		done <- true
	}()
	scannerError := bufio.NewScanner(cmdErrorReader)
	doneErr := make(chan bool)
	go func() {
		for scannerError.Scan() {
			output += scannerError.Text() + "\n"
			log.Error(scannerError.Text())
		}
		doneErr <- true
	}()
	err = cmd.Start()
	if err != nil {
		log.Warning("Error starting Cmd", err.Error())
		return ""
	}
	cmd.Stdin = strings.NewReader(content)
	<-done
	<-doneErr
	err = cmd.Wait()
	return output
}
