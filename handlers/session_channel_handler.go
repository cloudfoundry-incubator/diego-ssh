package handlers

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sync"
	"syscall"
	"time"

	"github.com/cloudfoundry-incubator/diego-ssh/helpers"
	"github.com/cloudfoundry-incubator/diego-ssh/scp"
	"github.com/docker/docker/pkg/term"
	"github.com/pivotal-golang/lager"
	"golang.org/x/crypto/ssh"
)

var scpRegex = regexp.MustCompile(`^\s*scp($|\s+)`)

//go:generate counterfeiter -o fakes/fake_runner.go . Runner
type Runner interface {
	Start(cmd *exec.Cmd) error
	Wait(cmd *exec.Cmd) error
	Signal(cmd *exec.Cmd, signal syscall.Signal) error
}

type commandRunner struct{}

func NewCommandRunner() Runner {
	return &commandRunner{}
}

func (commandRunner) Start(cmd *exec.Cmd) error {
	return cmd.Start()
}

func (commandRunner) Wait(cmd *exec.Cmd) error {
	return cmd.Wait()
}

func (commandRunner) Signal(cmd *exec.Cmd, signal syscall.Signal) error {
	return cmd.Process.Signal(signal)
}

//go:generate counterfeiter -o fakes/fake_shell_locator.go . ShellLocator
type ShellLocator interface {
	ShellPath() string
}

type shellLocator struct{}

func NewShellLocator() ShellLocator {
	return &shellLocator{}
}

func (shellLocator) ShellPath() string {
	for _, shell := range []string{"/bin/bash", "/usr/local/bin/bash", "/bin/sh", "bash", "sh"} {
		if path, err := exec.LookPath(shell); err == nil {
			return path
		}
	}

	return "/bin/sh"
}

type SessionChannelHandler struct {
	runner       Runner
	shellLocator ShellLocator
	defaultEnv   map[string]string
	keepalive    time.Duration
}

func NewSessionChannelHandler(
	runner Runner,
	shellLocator ShellLocator,
	defaultEnv map[string]string,
	keepalive time.Duration,
) *SessionChannelHandler {
	return &SessionChannelHandler{
		runner:       runner,
		shellLocator: shellLocator,
		defaultEnv:   defaultEnv,
		keepalive:    keepalive,
	}
}

func (handler *SessionChannelHandler) HandleNewChannel(logger lager.Logger, newChannel ssh.NewChannel) {
	channel, requests, err := newChannel.Accept()
	if err != nil {
		logger.Error("handle-new-session-channel-failed", err)
		return
	}

	handler.newSession(logger, channel, handler.keepalive).serviceRequests(requests)
}

type ptyRequestMsg struct {
	Term     string
	Columns  uint32
	Rows     uint32
	Width    uint32
	Height   uint32
	Modelist string
}

type session struct {
	logger            lager.Logger
	complete          bool
	keepaliveDuration time.Duration
	keepaliveStopCh   chan struct{}

	shellPath string
	runner    Runner
	channel   ssh.Channel

	sync.Mutex
	env     map[string]string
	command *exec.Cmd

	wg         sync.WaitGroup
	allocPty   bool
	ptyRequest ptyRequestMsg

	ptyMaster *os.File
}

func (handler *SessionChannelHandler) newSession(logger lager.Logger, channel ssh.Channel, keepalive time.Duration) *session {
	return &session{
		logger:            logger.Session("session-channel"),
		keepaliveDuration: keepalive,
		runner:            handler.runner,
		shellPath:         handler.shellLocator.ShellPath(),
		channel:           channel,
		env:               handler.defaultEnv,
	}
}

func (sess *session) handleEnvironmentRequest(request *ssh.Request) {
	logger := sess.logger.Session("handle-environment-request")

	type envMsg struct {
		Name  string
		Value string
	}
	var envMessage envMsg

	err := ssh.Unmarshal(request.Payload, &envMessage)
	if err != nil {
		logger.Error("unmarshal-failed", err)
		request.Reply(false, nil)
		return
	}

	sess.Lock()
	sess.env[envMessage.Name] = envMessage.Value
	sess.Unlock()

	if request.WantReply {
		request.Reply(true, nil)
	}
}

func (sess *session) handleSignalRequest(request *ssh.Request) {
	logger := sess.logger.Session("handle-signal-request")

	type signalMsg struct {
		Signal string
	}
	var signalMessage signalMsg

	err := ssh.Unmarshal(request.Payload, &signalMessage)
	if err != nil {
		logger.Error("unmarshal-failed", err)
		if request.WantReply {
			request.Reply(false, nil)
		}
		return
	}

	sess.Lock()
	defer sess.Unlock()

	cmd := sess.command

	if cmd != nil {
		signal := SyscallSignals[ssh.Signal(signalMessage.Signal)]
		err := sess.runner.Signal(cmd, signal)
		if err != nil {
			logger.Error("process-signal-failed", err)
		}
	}

	if request.WantReply {
		request.Reply(true, nil)
	}
}

func (sess *session) handlePtyRequest(request *ssh.Request) {
	logger := sess.logger.Session("handle-pty-request")

	var ptyRequestMessage ptyRequestMsg

	err := ssh.Unmarshal(request.Payload, &ptyRequestMessage)
	if err != nil {
		logger.Error("unmarshal-failed", err)
		if request.WantReply {
			request.Reply(false, nil)
		}
		return
	}

	sess.Lock()
	defer sess.Unlock()

	sess.allocPty = true
	sess.ptyRequest = ptyRequestMessage
	sess.env["TERM"] = ptyRequestMessage.Term

	if request.WantReply {
		request.Reply(true, nil)
	}
}

func (sess *session) handleWindowChangeRequest(request *ssh.Request) {
	logger := sess.logger.Session("handle-window-change")

	type windowChangeMsg struct {
		Columns  uint32
		Rows     uint32
		WidthPx  uint32
		HeightPx uint32
	}
	var windowChangeMessage windowChangeMsg

	err := ssh.Unmarshal(request.Payload, &windowChangeMessage)
	if err != nil {
		logger.Error("unmarshal-failed", err)
		if request.WantReply {
			request.Reply(false, nil)
		}
		return
	}

	sess.Lock()
	defer sess.Unlock()

	if sess.allocPty {
		sess.ptyRequest.Columns = windowChangeMessage.Columns
		sess.ptyRequest.Rows = windowChangeMessage.Rows
	}

	if sess.ptyMaster != nil {
		err = setWindowSize(logger, sess.ptyMaster, sess.ptyRequest.Columns, sess.ptyRequest.Rows)
		if err != nil {
			logger.Error("failed-to-set-window-size", err)
		}
	}

	if request.WantReply {
		request.Reply(true, nil)
	}
}

func (sess *session) handleExecRequest(request *ssh.Request) {
	logger := sess.logger.Session("handle-exec-request")

	type execMsg struct {
		Command string
	}
	var execMessage execMsg

	err := ssh.Unmarshal(request.Payload, &execMessage)
	if err != nil {
		logger.Error("unmarshal-failed", err)
		if request.WantReply {
			request.Reply(false, nil)
		}
		return
	}

	if scpRegex.MatchString(execMessage.Command) {
		logger.Info("handling-scp-command", lager.Data{"Command": execMessage.Command})
		sess.executeSCP(execMessage.Command, request)
	} else {
		sess.executeShell(request, "-c", execMessage.Command)
	}
}

func (sess *session) handleShellRequest(request *ssh.Request) {
	sess.executeShell(request)
}

func (sess *session) executeShell(request *ssh.Request, args ...string) {
	logger := sess.logger.Session("execute-shell")

	sess.Lock()
	cmd, err := sess.createCommand(args...)
	if err != nil {
		sess.Unlock()
		logger.Error("failed-to-create-command", err)
		if request.WantReply {
			request.Reply(false, nil)
		}
		return
	}

	if request.WantReply {
		request.Reply(true, nil)
	}

	if sess.allocPty {
		err = sess.runWithPty(cmd)
	} else {
		err = sess.run(cmd)
	}

	sess.Unlock()

	if err != nil {
		sess.sendExitMessage(err)
		sess.destroy()
		return
	}

	go func() {
		err := sess.wait(cmd)
		sess.sendExitMessage(err)
		sess.destroy()
	}()
}

func (sess *session) createCommand(args ...string) (*exec.Cmd, error) {
	if sess.command != nil {
		return nil, errors.New("command already started")
	}

	cmd := exec.Command(sess.shellPath, args...)
	cmd.Env = sess.environment()
	sess.command = cmd

	return cmd, nil
}

func (sess *session) environment() []string {
	env := []string{}

	env = append(env, "PATH=/bin:/usr/bin")
	env = append(env, "LANG=en_US.UTF8")

	for k, v := range sess.env {
		if k != "HOME" && k != "USER" {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	env = append(env, fmt.Sprintf("HOME=%s", os.Getenv("HOME")))
	env = append(env, fmt.Sprintf("USER=%s", os.Getenv("USER")))

	return env
}

type exitStatusMsg struct {
	Status uint32
}

type exitSignalMsg struct {
	Signal     string
	CoreDumped bool
	Error      string
	Lang       string
}

func (sess *session) sendExitMessage(err error) {
	logger := sess.logger.Session("send-exit-message")
	logger.Info("started")
	defer logger.Info("finished")

	if err != nil {
		logger.Error("building-exit-message-from-error", err)
	}

	if err == nil {
		_, sendErr := sess.channel.SendRequest("exit-status", false, ssh.Marshal(exitStatusMsg{}))
		if sendErr != nil {
			logger.Error("send-exit-status-failed", sendErr)
		}
		return
	}

	exitError, ok := err.(*exec.ExitError)
	if !ok {
		exitMessage := exitStatusMsg{Status: 255}
		_, sendErr := sess.channel.SendRequest("exit-status", false, ssh.Marshal(exitMessage))
		if sendErr != nil {
			logger.Error("send-exit-status-failed", sendErr)
		}
		return
	}

	waitStatus, ok := exitError.Sys().(syscall.WaitStatus)
	if !ok {
		exitMessage := exitStatusMsg{Status: 255}
		_, sendErr := sess.channel.SendRequest("exit-status", false, ssh.Marshal(exitMessage))
		if sendErr != nil {
			logger.Error("send-exit-status-failed", sendErr)
		}
		return
	}

	if waitStatus.Signaled() {
		exitMessage := exitSignalMsg{
			Signal:     string(SSHSignals[waitStatus.Signal()]),
			CoreDumped: waitStatus.CoreDump(),
		}
		_, sendErr := sess.channel.SendRequest("exit-signal", false, ssh.Marshal(exitMessage))
		if sendErr != nil {
			logger.Error("send-exit-status-failed", sendErr)
		}
		return
	}

	exitMessage := exitStatusMsg{Status: uint32(waitStatus.ExitStatus())}
	_, sendErr := sess.channel.SendRequest("exit-status", false, ssh.Marshal(exitMessage))
	if sendErr != nil {
		logger.Error("send-exit-status-failed", sendErr)
	}
}

func setWindowSize(logger lager.Logger, pseudoTty *os.File, columns, rows uint32) error {
	logger.Info("new-size", lager.Data{"columns": columns, "rows": rows})
	return term.SetWinsize(pseudoTty.Fd(), &term.Winsize{
		Width:  uint16(columns),
		Height: uint16(rows),
	})
}

func setTerminalAttributes(logger lager.Logger, pseudoTty *os.File, modelist string) {
	reader := bytes.NewReader([]byte(modelist))

	for {
		var opcode uint8
		var value uint32

		err := binary.Read(reader, binary.BigEndian, &opcode)
		if err != nil {
			logger.Error("failed-to-read-modelist-opcode", err)
			break
		}

		if opcode == 0 || opcode >= 160 {
			break
		}

		err = binary.Read(reader, binary.BigEndian, &value)
		if err != nil {
			logger.Error("failed-to-read-modelist-value", err)
			break
		}

		logger.Info("set-terminal-attribute", lager.Data{
			"opcode": opcode,
			"value":  fmt.Sprintf("%x", value),
		})

		termios, err := TcGetAttr(pseudoTty)
		if err != nil {
			logger.Error("failed-to-get-terminal-attrs", err)
			continue
		}

		err = TermAttrSetters[opcode].Set(pseudoTty, termios, value)
		if err != nil {
			logger.Error("failed-to-set-terminal-attrs", err, lager.Data{
				"opcode": opcode,
				"value":  fmt.Sprintf("%x", value),
			})
			continue
		}
	}
}

func (sess *session) run(command *exec.Cmd) error {
	logger := sess.logger.Session("run")

	command.Stdout = sess.channel
	command.Stderr = sess.channel.Stderr()

	stdin, err := command.StdinPipe()
	if err != nil {
		return err
	}

	go helpers.CopyAndClose(logger.Session("to-stdin"), nil, stdin, sess.channel)

	return sess.runner.Start(command)
}

func (sess *session) keepalive(command *exec.Cmd, stopCh chan struct{}) {
	logger := sess.logger.Session("keepalive")

	ticker := time.NewTicker(sess.keepaliveDuration)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			_, err := sess.channel.SendRequest("keepalive@cloudfoundry.org", true, nil)
			logger.Info("keepalive", lager.Data{"success": err == nil})

			if err != nil {
				err = sess.runner.Signal(command, syscall.SIGHUP)
				logger.Info("process-signaled", lager.Data{"error": err})
				return
			}
		case <-stopCh:
			return
		}
	}
}

func (sess *session) wait(command *exec.Cmd) error {
	logger := sess.logger.Session("wait")
	logger.Info("started")
	defer logger.Info("done")
	return sess.runner.Wait(command)
}

func (sess *session) destroy() {
	sess.Lock()
	defer sess.Unlock()

	if sess.complete {
		return
	}

	sess.wg.Wait()
	sess.complete = true

	if sess.channel != nil {
		sess.channel.Close()
	}

	if sess.ptyMaster != nil {
		sess.ptyMaster.Close()
		sess.ptyMaster = nil
	}

	if sess.keepaliveStopCh != nil {
		close(sess.keepaliveStopCh)
	}
}

func (sess *session) executeSCP(command string, request *ssh.Request) {
	logger := sess.logger.Session("execute-scp")

	if request.WantReply {
		request.Reply(true, nil)
	}

	copier, err := scp.NewFromCommand(command, sess.channel, sess.channel, sess.channel.Stderr(), logger)
	if err == nil {
		err = copier.Copy()
	}

	sess.sendSCPExitMessage(err)
	sess.destroy()
}

func (sess *session) sendSCPExitMessage(err error) {
	logger := sess.logger.Session("send-scp-exit-message")
	logger.Info("started")
	defer logger.Info("finished")

	var exitMessage exitStatusMsg
	if err != nil {
		logger.Error("building-scp-exit-message-from-error", err)
		exitMessage = exitStatusMsg{Status: 1}
	}

	_, sendErr := sess.channel.SendRequest("exit-status", false, ssh.Marshal(exitMessage))
	if sendErr != nil {
		logger.Error("send-exit-status-failed", sendErr)
	}
}
