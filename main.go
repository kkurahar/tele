package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/biosugar0/tele/params"
	"github.com/biosugar0/tele/pkg/util"
	"github.com/spf13/cobra"
)

var (
	versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Print the version number of tele",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("tele version %s\n", params.Version)
		},
	}

	rootCmd = &cobra.Command{
		Version: params.Version,
		Use:     `tele --run "<shell command>"`,
		Short:   "simple Telepresence wrapper tool for development microservices",
		Long: `A simple Telepresence wrapper tool for microservice development.

 Find more information at: https://github.com/biosugar0/tele
`,
		RunE: Run,
	}
)

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.SetOutput(os.Stdout)
}

func execute(cmdstr string) (string, error) {
	cmd := exec.Command("bash", "-c", cmdstr)
	cmd.Env = os.Environ()
	var sout bytes.Buffer
	var serr bytes.Buffer
	cmd.Stdout = &sout
	cmd.Stderr = &serr
	err := cmd.Run()
	if err != nil {
		return "", errors.New(serr.String())
	}
	result := strings.TrimRight(sout.String(), "\n")
	return result, nil
}

func executeStream(cmdstr string) error {
	cmd := exec.Command("bash", "-c", cmdstr)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	err = cmd.Start()
	if err != nil {
		return err
	}
	streamReader := func(scanner *bufio.Scanner, output chan string, done chan bool) {
		defer close(output)
		defer close(done)
		for scanner.Scan() {
			output <- scanner.Text()
		}
		done <- true
	}

	stdoutScanner := bufio.NewScanner(stdout)
	stdoutOutputChan := make(chan string)
	stdoutDoneChan := make(chan bool)
	stderrScanner := bufio.NewScanner(stderr)
	stderrOutputChan := make(chan string)
	stderrDoneChan := make(chan bool)
	go streamReader(stdoutScanner, stdoutOutputChan, stdoutDoneChan)
	go streamReader(stderrScanner, stderrOutputChan, stderrDoneChan)

	runnning := true
	for runnning {
		select {
		case <-stdoutDoneChan:
			runnning = false
		case line := <-stdoutOutputChan:
			fmt.Println(line)
		case line := <-stderrOutputChan:
			fmt.Println(line)
		}
	}

	err = cmd.Wait()
	if err != nil {
		return err
	}

	return nil
}

func Run(cmd *cobra.Command, args []string) error {
	repository, err := execute("git rev-parse --show-toplevel")
	if err != nil {
		return err
	}
	repo := filepath.Base(repository)
	cmd.Printf("[repository]:\n %s\n", repo)

	branch, err := execute(`git rev-parse --abbrev-ref @`)
	if err != nil {
		return err
	}
	cmd.Printf("[branch]:\n %s\n", branch)

	user := params.User
	cmd.Printf("[user]:\n %s\n", user)

	namespace := params.NameSpace
	cmd.Printf("[namespace]:\n %s\n", namespace)

	port := params.ServerPort
	cmd.Printf("[port]:\n %s\n", port)

	deployment := strings.Join([]string{
		user,
		repo,
		branch,
	}, "-")
	deployment = util.ToValidName(deployment)

	cmd.Printf("[deployment]:\n %s\n", deployment)

	run := params.CMD

	cmd.Printf("[request command]:\n %s\n", run)

	telepresence := fmt.Sprintf(
		"telepresence --namespace %s --method inject-tcp --new-deployment %s",
		namespace,
		deployment,
	)
	if len(port) > 0 {
		telepresence += fmt.Sprintf(" --expose %s", port)
	}
	telepresence += fmt.Sprintf(
		" --run bash -c \"%s\"",
		run,
	)

	cmd.Printf("[Telepreesence command]:\n %s\n", telepresence)

	cmd.Printf("[result]:\n")
	err = executeStream(telepresence)
	if err != nil {
		return err
	}

	return nil
}

func main() {
	homedir := filepath.Base(os.Getenv("HOME"))
	rootCmd.PersistentFlags().SortFlags = false
	rootCmd.PersistentFlags().StringVar(&params.CMD, "run", "echo hello world", "shell command")
	rootCmd.PersistentFlags().StringVar(&params.ServerPort, "port", "", "expose http server port")
	rootCmd.PersistentFlags().StringVar(&params.User, "user", homedir, "user name for prefix of deployment name. default is home directory name")
	rootCmd.PersistentFlags().StringVar(&params.NameSpace, "namespace", "default", "name space of kubernetes")
	if err := rootCmd.Execute(); err != nil {
		rootCmd.SetOutput(os.Stderr)
		rootCmd.Println(err)
		os.Exit(1)
	}
}
