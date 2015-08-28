// +build windows

package main_test

import (
	"fmt"
	"io/ioutil"
	"time"

	"github.com/cloudfoundry-incubator/diego-ssh/cmd/sshd/testrunner"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
	"golang.org/x/crypto/ssh"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("SSH daemon", func() {
	var (
		runner  ifrit.Runner
		process ifrit.Process

		address       string
		hostKey       string
		privateKey    string
		authorizedKey string

		allowUnauthenticatedClients bool
		inheritDaemonEnv            bool
	)

	BeforeEach(func() {
		hostKey = hostKeyPem
		privateKey = privateKeyPem
		authorizedKey = publicAuthorizedKey

		allowUnauthenticatedClients = false
		inheritDaemonEnv = false
		address = fmt.Sprintf("127.0.0.1:%d", sshdPort)
	})

	JustBeforeEach(func() {
		args := testrunner.Args{
			Address:       address,
			HostKey:       string(hostKey),
			AuthorizedKey: string(authorizedKey),

			AllowUnauthenticatedClients: allowUnauthenticatedClients,
			InheritDaemonEnv:            inheritDaemonEnv,
		}

		runner = testrunner.New(sshdPath, args)
		process = ifrit.Invoke(runner)
	})

	AfterEach(func() {
		ginkgomon.Kill(process, 3*time.Second)
	})

	Describe("SSH features", func() {
		var clientConfig *ssh.ClientConfig
		var client *ssh.Client

		BeforeEach(func() {
			allowUnauthenticatedClients = true
			clientConfig = &ssh.ClientConfig{}
		})

		JustBeforeEach(func() {
			Expect(process).NotTo(BeNil())

			var dialErr error
			client, dialErr = ssh.Dial("tcp", address, clientConfig)
			Expect(dialErr).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			client.Close()
		})

		Context("when a client requests the execution of a command", func() {
			It("runs the command", func() {
				session, err := client.NewSession()
				Expect(err).NotTo(HaveOccurred())

				// result, err := session.Output("/bin/echo -n 'Hello there!'")
				// Expect(err).NotTo(HaveOccurred())

				stderr, err := session.StderrPipe()
				stderrBytes, err := ioutil.ReadAll(stderr)
				Expect(string(stderrBytes)).To(Equal("SSH is not supported on Windows cells\n"))
			})
		})
	})
})
