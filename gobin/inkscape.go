package gobin

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
)

func (s *Server) convertSVG2PNG(svg string) ([]byte, error) {
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)

	dpi := 96
	if s.cfg.Preview.DPI > 0 {
		dpi = s.cfg.Preview.DPI
	}

	cmd := exec.Command(s.cfg.Preview.InkscapePath, "-p", "-d", strconv.Itoa(dpi), "--convert-dpi-method=scale-viewbox", "--export-filename=-", "--export-type=png")
	cmd.Stdin = bytes.NewReader([]byte(svg))
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("error while converting svg: %s %w", stderr.String(), err)
	}

	if stdout.Len() == 0 {
		return nil, fmt.Errorf("no data from inkscape")
	}

	return stdout.Bytes(), nil
}
