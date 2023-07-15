package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
)

func CopyWithLogging(dst io.Writer, src io.Reader) (int64, error) {
	buffer := make([]byte, 4096)
	var totalBytes int64
	for {
		n, err := src.Read(buffer)
		if n > 0 {
			log.Printf("Read %d bytes: %s\n", n, buffer[:n])
			written, writeErr := dst.Write(buffer[:n])
			if written > 0 {
				log.Printf("Written %d bytes: %s\n", written, buffer[:written])
			}
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
	// go CopyWithLogging(src, dst)
	// CopyWithLogging(dst, src)
	go io.Copy(src, dst)
	io.Copy(dst, src)
}

func NewConfig() *tls.Config {
	cert, err := tls.LoadX509KeyPair("server.crt", "server.key")
	if err != nil {
		log.Panicf("Load err: %v", err)
	}
	config := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: true,
	}
	return config
}

func handleHttps(w http.ResponseWriter, r *http.Request) {
	remoteConn, err := tls.Dial("tcp", r.Host, &tls.Config{
		InsecureSkipVerify: true,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking fail", http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	TLSclientConn := tls.Server(clientConn, NewConfig())
	transmit(remoteConn, TLSclientConn)
}

func handleHttp(w http.ResponseWriter, r *http.Request) {
	resp, err := http.DefaultTransport.RoundTrip(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()
	for key, vals := range resp.Header {
		for _, val := range vals {
			w.Header().Add(key, val)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func main() {
	server := &http.Server{
		Addr:      "127.0.0.1:8080",
		TLSConfig: NewConfig(),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodConnect {
				fmt.Printf("HTTPS Connection\n")
				handleHttps(w, r)
			} else {
				fmt.Printf("HTTP Connection\n")
				handleHttp(w, r)
			}
		}),
	}
	log.Fatal(server.ListenAndServeTLS("server.crt", "server.key"))
}
