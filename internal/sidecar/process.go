package sidecar

import (
	"context"
	"io"
	"os/exec"
)

type Process struct {
	Name string
	Path string
	Args []string
}

func (p Process) Command(ctx context.Context) *exec.Cmd {
	return exec.CommandContext(ctx, p.Path, p.Args...)
}

func (p Process) RunForeground(ctx context.Context, stdout io.Writer, stderr io.Writer) error {
	cmd := p.Command(ctx)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

func (p Process) Start(ctx context.Context, stdout io.Writer, stderr io.Writer) (*exec.Cmd, error) {
	cmd := p.Command(ctx)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}
