package env

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
)

func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: r}
}

type Decoder struct {
	r io.Reader
}

func (d *Decoder) Decode(v *map[string]string) error {
	r := bufio.NewReader(d.r)

	for {
		line, _, err := r.ReadLine()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		if len(line) == 0 {
			continue
		}

		if line[0] == '#' {
			continue
		}

		parts := bytes.SplitN(line, []byte("="), 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid line: %s", line)
		}

		(*v)[string(parts[0])] = string(parts[1])
	}

	return nil
}

func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w: w}
}

type Encoder struct {
	w io.Writer
}

func (e *Encoder) Encode(v map[string]string) error {
	for key, value := range v {
		_, err := fmt.Fprintf(e.w, "%s=%s\n", key, value)
		if err != nil {
			return err
		}
	}
	return nil
}
