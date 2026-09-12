package service

import (
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"ops-admin/backend/model"
)

func (s *Service) newSSHClient(host model.AssetHost) (*ssh.Client, error) {
	return s.newSSHClientWithTimeout(host, 5*time.Second)
}

func (s *Service) newSSHClientWithTimeout(host model.AssetHost, timeout time.Duration) (*ssh.Client, error) {
	if host.SSHIP == "" {
		return nil, errors.New("host has no SSH address configured")
	}
	if host.SSHUser == "" {
		return nil, errors.New("host has no SSH user configured")
	}
	authMethod, err := credentialAuthMethod(host.Credential)
	if err != nil {
		return nil, err
	}
	if timeout <= 0 {
		return nil, errors.New("host synchronization timed out")
	}
	config := &ssh.ClientConfig{
		User:            host.SSHUser,
		Auth:            []ssh.AuthMethod{authMethod},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         timeout,
	}
	address := net.JoinHostPort(host.SSHIP, strconv.Itoa(host.SSHPort))
	if normalizeConnectionMode(host.ConnectionMode) == "gateway" && host.GatewayID != nil && *host.GatewayID > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		conn, cleanup, err := s.dialThroughGateway(ctx, *host.GatewayID, "tcp", address)
		cancel()
		if err != nil {
			return nil, err
		}
		_ = conn.SetDeadline(time.Now().Add(timeout))
		clientConn, chans, reqs, err := ssh.NewClientConn(conn, address, config)
		if err != nil {
			_ = conn.Close()
			cleanup()
			return nil, err
		}
		_ = conn.SetDeadline(time.Time{})
		client := ssh.NewClient(clientConn, chans, reqs)
		go func() {
			_ = clientConn.Wait()
			cleanup()
		}()
		return client, nil
	}
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return nil, err
	}
	_ = conn.SetDeadline(time.Now().Add(timeout))
	clientConn, chans, reqs, err := ssh.NewClientConn(conn, address, config)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	_ = conn.SetDeadline(time.Time{})
	return ssh.NewClient(clientConn, chans, reqs), nil
}

func credentialAuthMethod(credential model.AssetCredential) (ssh.AuthMethod, error) {
	switch normalizedAuthType(credential.AuthType) {
	case "key":
		var signer ssh.Signer
		var err error
		if strings.TrimSpace(credential.Passphrase) != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(credential.PrivateKey), []byte(credential.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey([]byte(credential.PrivateKey))
		}
		if err != nil {
			return nil, err
		}
		return ssh.PublicKeys(signer), nil
	default:
		if credential.Password == "" {
			return nil, errors.New("password credential is empty")
		}
		return ssh.Password(credential.Password), nil
	}
}

func remainingTimeout(deadline time.Time, maximum time.Duration) time.Duration {
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return 0
	}
	if remaining < maximum {
		return remaining
	}
	return maximum
}

func collectHostInfo(client *ssh.Client, deadline time.Time) (map[string]string, bool) {
	result := map[string]string{}
	commands := map[string]string{
		"os":         ". /etc/os-release 2>/dev/null && printf '%s' \"$PRETTY_NAME\" || lsb_release -ds 2>/dev/null || uname -sr 2>/dev/null || true",
		"arch":       "uname -m 2>/dev/null || true",
		"cpu":        "nproc 2>/dev/null || grep -c processor /proc/cpuinfo 2>/dev/null || true",
		"memory":     "free -m 2>/dev/null | awk '/Mem:/ {printf \"%dG\", int(($2 + 1023) / 1024)}' || true",
		"disk":       "df -h / 2>/dev/null | awk 'NR==2 {print $2}' || true",
		"private_ip": "hostname -I 2>/dev/null | awk '{print $1}' || true",
		"public_ip":  "curl -s --max-time 2 ifconfig.me 2>/dev/null || wget -qO- -T 2 ifconfig.me 2>/dev/null || true",
	}
	for field, command := range commands {
		timeout := remainingTimeout(deadline, 5*time.Second)
		if timeout <= 0 {
			return result, true
		}
		value, timedOut := runSSHCommandWithTimeout(client, command, timeout)
		if timedOut {
			return result, true
		}
		if field == "cpu" && value != "" {
			value = value + " cores"
		}
		result[field] = value
	}
	return result, false
}

func runSSHCommand(client *ssh.Client, command string) string {
	output, _ := runSSHCommandWithTimeout(client, command, 5*time.Second)
	return output
}

func runSSHCommandWithTimeout(client *ssh.Client, command string, timeout time.Duration) (string, bool) {
	session, err := client.NewSession()
	if err != nil {
		return "", false
	}
	defer session.Close()
	type commandResult struct {
		output []byte
		err    error
	}
	done := make(chan commandResult, 1)
	go func() {
		output, runErr := session.CombinedOutput(command)
		done <- commandResult{output: output, err: runErr}
	}()
	select {
	case result := <-done:
		if result.err != nil {
			return "", false
		}
		return strings.TrimSpace(string(result.output)), false
	case <-time.After(timeout):
		_ = session.Close()
		return "", true
	}
}

func formatConfig(host model.AssetHost) string {
	parts := make([]string, 0, 3)
	if host.CPU != "" {
		parts = append(parts, host.CPU)
	}
	if host.Memory != "" {
		parts = append(parts, host.Memory)
	}
	if host.Disk != "" {
		parts = append(parts, host.Disk)
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, " / ")
}
