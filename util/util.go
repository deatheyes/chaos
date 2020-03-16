package util

import (
	"bytes"
	"io/ioutil"
	"os/exec"
	"strings"

	"github.com/pborman/uuid"
)

// NewUUID create unique id
func NewUUID() string {
	return uuid.NewUUID().String()
}

// RunCommand execute a command as shell
func RunCommand(command string) ([]byte, error) {
	var out bytes.Buffer
	cmd := exec.Command("/bin/bash", "-c", command)
	cmd.Stdout = &out
	err := cmd.Run()
	return out.Bytes(), err
}

// StartCommand execute a command
func StartCommand(command string) error {
	cmd := exec.Command("/bin/bash", "-c", command)
	return cmd.Start()
}

// GetPidFromfile read pid from file
func GetPidFromfile(file string) (string, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return "", err
	}
	return strings.Trim(string(data), " \n\t"), nil
}

// FilterNodes filter nodes for safe
func FilterNodes(Reserved, agents []string) []string {
	ret := []string{}
	for _, a := range agents {
		flag := false
		for _, f := range agents {
			prefix := f + ":"
			if strings.HasPrefix(a, prefix) {
				flag = true
				break
			}
		}
		if !flag {
			ret = append(ret, a)
		}
	}
	return ret
}

// ListAllFileByName list all the file with specified suffix
func ListAllFileByName(dir, suffix string) ([]string, error) {
	ret := []string{}
	list, err := ioutil.ReadDir(dir)
	if err != nil {
		return ret, nil
	}

	for _, item := range list {
		if !item.IsDir() {
			if strings.HasSuffix(item.Name(), suffix) {
				ret = append(ret, item.Name())
			}
		}
	}
	return ret, nil
}
