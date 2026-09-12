package service

import (
	"io"

	"golang.org/x/crypto/ssh"
	"ops-admin/backend/model"
)

type AssetTerminalSession struct {
	Host    model.AssetHost
	Client  *ssh.Client
	Session *ssh.Session
	Stdin   io.WriteCloser
	Stdout  io.Reader
	Stderr  io.Reader
}

func (s *Service) OpenAssetTerminal(id uint, rows, cols int) (*AssetTerminalSession, error) {
	var host model.AssetHost
	if err := s.db.Preload("Credential").Preload("Gateway").Preload("Gateway.Credential").First(&host, id).Error; err != nil {
		return nil, err
	}
	if rows <= 0 {
		rows = 30
	}
	if cols <= 0 {
		cols = 120
	}

	client, err := s.newSSHClient(host)
	if err != nil {
		return nil, err
	}
	session, err := client.NewSession()
	if err != nil {
		client.Close()
		return nil, err
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		session.Close()
		client.Close()
		return nil, err
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		session.Close()
		client.Close()
		return nil, err
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		session.Close()
		client.Close()
		return nil, err
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := session.RequestPty("xterm-256color", rows, cols, modes); err != nil {
		session.Close()
		client.Close()
		return nil, err
	}
	if err := session.Shell(); err != nil {
		session.Close()
		client.Close()
		return nil, err
	}

	return &AssetTerminalSession{
		Host:    host,
		Client:  client,
		Session: session,
		Stdin:   stdin,
		Stdout:  stdout,
		Stderr:  stderr,
	}, nil
}
