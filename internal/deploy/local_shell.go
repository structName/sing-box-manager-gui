package deploy

import (
	"bytes"
	"os"
	"os/exec"
)

func RunLocalShell(path string, env map[string]string) (string, error) {
	cmd := exec.Command("bash", path)
	cmd.Env = os.Environ()
	for key, value := range env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := cmd.Run()
	return output.String(), err
}
