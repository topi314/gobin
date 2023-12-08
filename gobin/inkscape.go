package gobin

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func (s *Server) convertSVG2PNG(ctx context.Context, svg string) ([]byte, error) {
	ctx, span := s.tracer.Start(ctx, "convertSVG2PNG", trace.WithAttributes(attribute.String("inkscape", s.cfg.Preview.InkscapePath)))
	defer span.End()

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)

	dpi := 96
	if s.cfg.Preview.DPI > 0 {
		dpi = s.cfg.Preview.DPI
	}
	span.SetAttributes(attribute.Int("dpi", dpi))

	cmd := exec.CommandContext(ctx, s.cfg.Preview.InkscapePath, "-p", "-d", strconv.Itoa(dpi), "--convert-dpi-method=scale-viewbox", "--export-filename=-", "--export-type=png")
	cmd.Stdin = bytes.NewReader([]byte(svg))
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		span.SetStatus(codes.Error, "failed to convert svg to png")
		span.RecordError(err)
		return nil, fmt.Errorf("error while converting svg: %s %w", stderr.String(), err)
	}

	if stdout.Len() == 0 {
		err := errors.New("no data from inkscape")
		span.SetStatus(codes.Error, "failed to convert svg to png")
		span.RecordError(err)
		return nil, err
	}

	return stdout.Bytes(), nil
}
