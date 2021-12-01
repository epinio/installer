package exec

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/codeskyblue/kexec"
	"github.com/pkg/errors"
)

type ExternalFuncWithString func() (output string, err error)

type ExternalFunc func() (err error)

func RunProc(dir string, toStdout bool, cmd string, args ...string) (string, error) {
	if os.Getenv("DEBUG") == "true" {
		fmt.Printf("Executing: %s %v (in: %s)\n", cmd, args, dir)
	}
	p := kexec.Command(cmd, args...)

	var b bytes.Buffer
	if toStdout {
		p.Stdout = io.MultiWriter(os.Stdout, &b)
		p.Stderr = io.MultiWriter(os.Stderr, &b)
	} else {
		p.Stdout = &b
		p.Stderr = &b
	}

	p.Dir = dir

	if err := p.Run(); err != nil {
		return b.String(), err
	}

	err := p.Wait()
	return b.String(), err
}

func RunProcNoErr(dir string, toStdout bool, cmd string, args ...string) (string, error) {
	if os.Getenv("DEBUG") == "true" {
		fmt.Printf("Executing %s %v\n", cmd, args)
	}
	p := kexec.Command(cmd, args...)

	var b bytes.Buffer
	if toStdout {
		p.Stdout = io.MultiWriter(os.Stdout, &b)
		p.Stderr = nil
	} else {
		p.Stdout = &b
		p.Stderr = nil
	}

	p.Dir = dir

	if err := p.Run(); err != nil {
		return b.String(), err
	}

	err := p.Wait()
	return b.String(), err
}

// CreateTmpFile creates a temporary file on the disk with the given contents
// and returns the path to it and an error if something goes wrong.
func CreateTmpFile(contents string) (string, error) {
	tmpfile, err := ioutil.TempFile("", "epinio")
	if err != nil {
		return tmpfile.Name(), err
	}
	if _, err := tmpfile.Write([]byte(contents)); err != nil {
		return tmpfile.Name(), err
	}
	if err := tmpfile.Close(); err != nil {
		return tmpfile.Name(), err
	}

	return tmpfile.Name(), nil
}

// Kubectl invokes the `kubectl` command in PATH, running the specified command.
// It returns the command output and/or error.
func Kubectl(command ...string) (string, error) {
	_, err := exec.LookPath("kubectl")
	if err != nil {
		return "", errors.Wrap(err, "kubectl not in path")
	}

	currentdir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	return RunProc(currentdir, false, "kubectl", command...)
}
