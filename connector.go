package securelink

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

// NewHTTPSConnector provides a HTTP/S client with custom root CA and with the
// given client certificate
func NewHTTPSConnector(host string, cert *Certificate) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
				DualStack: true,
			}).DialContext,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,

			TLSClientConfig: GetBaseTLSConfig(host, cert),
		},
	}
}

// GetBaseTLSConfig returns a TLS configuration with the given certificate as
// "Certificate" and setup the "RootCAs" with the given certificate CertPool
func GetBaseTLSConfig(host string, cert *Certificate) *tls.Config {
	return &tls.Config{
		ServerName:   host,
		Certificates: []tls.Certificate{cert.GetTLSCertificate()},
		RootCAs:      cert.GetCertPool(),
		ClientCAs:    cert.GetCertPool(),
		ClientAuth:   tls.RequireAndVerifyClientCert,
		CurvePreferences: []tls.CurveID{
			tls.X25519,
			tls.CurveP384,
			tls.CurveP521,
			tls.CurveP256,
		},
		MinVersion: tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
		},
	}
}

// NewServiceConnector opens a new connection to the given address. Check the given hostname
// is the one returned by the server. The connection send the given certificate as client
// authentication. The timeout kill the connection after the given duration.
func NewServiceConnector(addr, host string, cert *Certificate, timeout time.Duration) (net.Conn, error) {
	tlsConfig := GetBaseTLSConfig(host, cert)

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return nil, err
	}

	err = conn.SetDeadline(time.Now().Add(timeout))
	if err != nil {
		return nil, err
	}

	tc, _ := newTransportConn(conn, false)

	return tc, nil
}
