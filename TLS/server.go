package TLS

import (
	"HTTP"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/big"
	"net"
	"time"
)

func forward(dst io.Writer, src io.Reader) (int64, error) {
	buffer := make([]byte, 4096)
	var totalBytes int64
	for {
		n, err := src.Read(buffer)
		if n > 0 {
			fmt.Printf("Read %d bytes: %s\n", n, string(buffer[:n]))
			written, writeErr := dst.Write(buffer[:n])
			if writeErr != nil {
				return totalBytes, writeErr
			}
			totalBytes += int64(written)
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return totalBytes, err
		}
	}
	return totalBytes, nil
}

func transmit(dst, src net.Conn) {
	defer dst.Close()
	defer src.Close()
	// go io.Copy(src, dst)
	// io.Copy(dst, src)
	go HTTP.ForwardRemote(src, dst)
	HTTP.ForwardClient(src, dst)
}

func NewRemoteConfig(host string) *tls.Config {
	cert, err := generateCert(host)
	if err != nil {
		log.Panicf("Load err: %v", err)
	}
	config := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}
	return config
}

func NewLocalConfig() *tls.Config {
	cert, err := tls.LoadX509KeyPair("../Certificate/server.crt", "../Certificate/server.key")
	if err != nil {
		log.Panicf("Load err: %v", err)
	}
	config := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: true,
	}
	return config
}

func handleTLS(conn net.Conn, host string) {
	hostAddr, _, _ := net.SplitHostPort(host)
	remoteConn, err := tls.Dial("tcp", host, NewRemoteConfig(hostAddr))
	if err != nil {
		log.Panic(err)
		return
	}
	defer remoteConn.Close()
	TLSconn := tls.Server(conn, NewRemoteConfig(hostAddr))
	err = TLSconn.Handshake()
	if err != nil {
		log.Panicf("TLS handshake err: %v\n", err)
		return
	}
	transmit(remoteConn, TLSconn)
}

func generateCert(host string) (cert tls.Certificate, err error) {
	rawCert, rawKey, err := generateKeyPair(host)
	if err != nil {
		return
	}
	return tls.X509KeyPair(rawCert, rawKey)
}

func generateKeyPair(host string) (rawCert, rawKey []byte, err error) {
	pri, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Panic(err)
	}
	validTime := time.Hour * 24 * 365 * 10
	notBefore := time.Now()
	notAfter := notBefore.Add(validTime)
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		log.Panicf("Rand err: %v\n", err)
		return
	}
	serverCert := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Country:            []string{"CN"},
			Province:           []string{"Shanghai"},
			Locality:           []string{"Shanghai"},
			Organization:       []string{"SJTU"},
			OrganizationalUnit: []string{"ACM"},
			CommonName:         host,
		},
		DNSNames:              []string{host},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	rootCertBytes, _ := ioutil.ReadFile("../Certificate/ca.crt")
	block, _ := pem.Decode(rootCertBytes)
	rootCert, _ := x509.ParseCertificate(block.Bytes)
	rootKeyBytes, _ := ioutil.ReadFile("../Certificate/ca.key")
	block, _ = pem.Decode(rootKeyBytes)
	rootKey, _ := x509.ParsePKCS8PrivateKey(block.Bytes)
	derBytes, err := x509.CreateCertificate(rand.Reader, &serverCert,
		rootCert, &pri.PublicKey, rootKey)
	if err != nil {
		log.Panicf("Create error: %v\n", err)
		return
	}
	rawCert = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	rawKey = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(pri)})
	return
}

func StartProxyListen() net.Listener {
	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		fmt.Printf("Listen TLS error: %v\n", err)
		return nil
	}
	return lis
}

func StartProxyHandle(lis net.Listener, host string) {
	conn, err := lis.Accept()
	if err != nil {
		fmt.Printf("Accept TLS error: %v\n", err)
		return
	}
	handleTLS(conn, host)
}
