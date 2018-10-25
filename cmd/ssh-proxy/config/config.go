package config

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"os"

	"code.cloudfoundry.org/debugserver"
	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/durationjson"
	"code.cloudfoundry.org/lager/lagerflags"
)

type SSHProxyConfig struct {
	lagerflags.LagerConfig
	debugserver.DebugServerConfig
	Address                         string                `json:"address,omitempty"`
	HealthCheckAddress              string                `json:"health_check_address,omitempty"`
	HostKey                         string                `json:"host_key"`
	BBSAddress                      string                `json:"bbs_address"`
	CCAPIURL                        string                `json:"cc_api_url"`
	UAATokenURL                     string                `json:"uaa_token_url"`
	UAAPassword                     string                `json:"uaa_password"`
	UAAUsername                     string                `json:"uaa_username"`
	UAACACert                       string                `json:"uaa_ca_cert"`
	SkipCertVerify                  bool                  `json:"skip_cert_verify"`
	EnableCFAuth                    bool                  `json:"enable_cf_auth"`
	EnableConsulServiceRegistration bool                  `json:"enable_consul_service_registration,omitempty"`
	EnableDiegoAuth                 bool                  `json:"enable_diego_auth"`
	DiegoCredentials                string                `json:"diego_credentials"`
	BBSCACert                       string                `json:"bbs_ca_cert"`
	BBSClientCert                   string                `json:"bbs_client_cert"`
	BBSClientKey                    string                `json:"bbs_client_key"`
	BBSClientSessionCacheSize       int                   `json:"bbs_client_session_cache_size"`
	BBSMaxIdleConnsPerHost          int                   `json:"bbs_max_idle_conns_per_host"`
	ConsulCluster                   string                `json:"consul_cluster"`
	AllowedCiphers                  string                `json:"allowed_ciphers"`
	AllowedMACs                     string                `json:"allowed_macs"`
	AllowedKeyExchanges             string                `json:"allowed_key_exchanges"`
	LoggregatorConfig               loggingclient.Config  `json:"loggregator"`
	CommunicationTimeout            durationjson.Duration `json:"communication_timeout,omitempty"`
	IdleConnectionTimeout           durationjson.Duration `json:"idle_connection_timeout,omitempty"`
	ConnectToInstanceAddress        bool                  `json:"connect_to_instance_address"`

	BackendTLSEnabled    bool     `json:"backend_tls_enabled,omitempty"`
	BackendTLSCACerts    []string `json:"backend_tls_ca_certs,omitempty"`
	BackendTLSClientCert string   `json:"backend_tls_client_cert,omitempty"`
	BackendTLSClientKey  string   `json:"backend_tls_client_key,omitempty"`
}

func NewSSHProxyConfig(configPath string) (SSHProxyConfig, error) {
	proxyConfig := SSHProxyConfig{}

	configFile, err := os.Open(configPath)
	if err != nil {
		return SSHProxyConfig{}, err
	}

	defer configFile.Close()

	decoder := json.NewDecoder(configFile)

	err = decoder.Decode(&proxyConfig)
	if err != nil {
		return SSHProxyConfig{}, err
	}

	return proxyConfig, nil
}

func (c SSHProxyConfig) BackendTLSConfig() (*tls.Config, error) {
	if !c.BackendTLSEnabled {
		return nil, nil
	}

	tlsConfig := tls.Config{
		RootCAs: x509.NewCertPool(),
	}
	for i, certStr := range c.BackendTLSCACerts {
		ok := tlsConfig.RootCAs.AppendCertsFromPEM([]byte(certStr))
		if !ok {
			return nil, fmt.Errorf("Failed to parse cert %d of BackendTLSCACerts", i)
		}
	}
	clientCertificate, err := tls.X509KeyPair([]byte(c.BackendTLSClientCert), []byte(c.BackendTLSClientKey))
	if err != nil {
		return nil, fmt.Errorf("Failed to parse client cert: %s", err)
	}
	tlsConfig.Certificates = []tls.Certificate{clientCertificate}

	return &tlsConfig, nil
}
