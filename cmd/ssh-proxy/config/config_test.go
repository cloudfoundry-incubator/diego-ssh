package config_test

import (
	"io/ioutil"
	"os"
	"time"

	"code.cloudfoundry.org/debugserver"
	"code.cloudfoundry.org/diego-ssh/cmd/ssh-proxy/config"
	"code.cloudfoundry.org/durationjson"
	"code.cloudfoundry.org/lager/lagerflags"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("SSHProxyConfig", func() {
	var configFilePath, configData string

	BeforeEach(func() {
		configData = `{
			"address": "1.1.1.1",
			"health_check_address": "2.2.2.2",
			"host_key": "I am a host key.",
			"bbs_address": "3.3.3.3",
			"cc_api_url": "4.4.4.4",
			"uaa_token_url": "5.5.5.5",
			"uaa_password": "uaa-password",
			"uaa_username": "uaa-username",
			"skip_cert_verify": true,
			"communication_timeout": "5s",
			"enable_cf_auth": true,
			"enable_consul_service_registration": true,
			"enable_diego_auth": true,
			"diego_credentials": "diego-password",
			"bbs_ca_cert": "I am a bbs ca cert.",
			"bbs_client_cert": "I am a bbs client cert.",
			"bbs_client_key": "I am a bbs client key.",
			"bbs_client_session_cache_size": 10,
			"bbs_max_idle_conns_per_host": 20,
			"consul_cluster": "I am a consul cluster.",
			"allowed_ciphers": "cipher1,cipher2,cipher3",
			"allowed_macs": "mac1,mac2,mac3",
			"allowed_key_exchanges": "exchange1,exchange2,exchange3",
			"log_level": "debug",
			"debug_address": "5.5.5.5:9090",
			"connect_to_instance_address": true,
			"idle_connection_timeout": "5ms",

			"backend_tls_enabled": true,
			"backend_tls_ca_certs": ["cert1", "cert2"],
			"backend_tls_client_cert": "I am a proxy client cert.",
			"backend_tls_client_key": "I am a proxy client key."
		}`
	})

	JustBeforeEach(func() {
		configFile, err := ioutil.TempFile("", "ssh-proxy-config")
		Expect(err).NotTo(HaveOccurred())

		n, err := configFile.WriteString(configData)
		Expect(err).NotTo(HaveOccurred())
		Expect(n).To(Equal(len(configData)))

		err = configFile.Close()
		Expect(err).NotTo(HaveOccurred())

		configFilePath = configFile.Name()
	})

	AfterEach(func() {
		err := os.RemoveAll(configFilePath)
		Expect(err).NotTo(HaveOccurred())
	})

	It("correctly parses the config file", func() {
		proxyConfig, err := config.NewSSHProxyConfig(configFilePath)
		Expect(err).NotTo(HaveOccurred())

		Expect(proxyConfig).To(Equal(config.SSHProxyConfig{
			Address:                         "1.1.1.1",
			HealthCheckAddress:              "2.2.2.2",
			HostKey:                         "I am a host key.",
			BBSAddress:                      "3.3.3.3",
			CCAPIURL:                        "4.4.4.4",
			UAATokenURL:                     "5.5.5.5",
			UAAPassword:                     "uaa-password",
			UAAUsername:                     "uaa-username",
			SkipCertVerify:                  true,
			CommunicationTimeout:            durationjson.Duration(5 * time.Second),
			EnableCFAuth:                    true,
			EnableConsulServiceRegistration: true,
			EnableDiegoAuth:                 true,
			DiegoCredentials:                "diego-password",
			BBSCACert:                       "I am a bbs ca cert.",
			BBSClientCert:                   "I am a bbs client cert.",
			BBSClientKey:                    "I am a bbs client key.",
			BBSClientSessionCacheSize:       10,
			BBSMaxIdleConnsPerHost:          20,
			ConsulCluster:                   "I am a consul cluster.",
			AllowedCiphers:                  "cipher1,cipher2,cipher3",
			AllowedMACs:                     "mac1,mac2,mac3",
			AllowedKeyExchanges:             "exchange1,exchange2,exchange3",
			ConnectToInstanceAddress:        true,
			IdleConnectionTimeout:           durationjson.Duration(5 * time.Millisecond),
			LagerConfig: lagerflags.LagerConfig{
				LogLevel: lagerflags.DEBUG,
			},
			DebugServerConfig: debugserver.DebugServerConfig{
				DebugAddress: "5.5.5.5:9090",
			},

			BackendTLSEnabled:    true,
			BackendTLSCACerts:    []string{"cert1", "cert2"},
			BackendTLSClientCert: "I am a proxy client cert.",
			BackendTLSClientKey:  "I am a proxy client key.",
		}))
	})

	Context("when the file does not exist", func() {
		It("returns an error", func() {
			_, err := config.NewSSHProxyConfig("foobar")
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when the file does not contain valid json", func() {
		BeforeEach(func() {
			configData = "{{"
		})

		It("returns an error", func() {
			_, err := config.NewSSHProxyConfig(configFilePath)
			Expect(err).To(HaveOccurred())
		})

		Context("because the communication_timeout is not valid", func() {
			BeforeEach(func() {
				configData = `{"communication_timeout": 4234342342}`
			})

			It("returns an error", func() {
				_, err := config.NewSSHProxyConfig(configFilePath)
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
