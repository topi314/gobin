package gobin

import (
	"bytes"
	"fmt"
	"os/exec"
)

func (s *Server) convertSVG2PNG(svg string) ([]byte, error) {
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)

	cmd := exec.Command(s.cfg.Preview.InkscapePath, "-p", "--export-filename=-", "--export-type=png")
	cmd.Stdin = bytes.NewReader([]byte(svg))
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("error while converting scg: %s %w", stderr.String(), err)
	}

	if stdout.Len() == 0 {
		return nil, fmt.Errorf("no data from inkscape")
	}

	return stdout.Bytes(), nil
}
