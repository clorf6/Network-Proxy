package HTTP

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
)

const (
	Unknown int = 0
	GET     int = 1
	HEAD    int = 2
	POST    int = 3
	PUT     int = 4
	DELETE  int = 5
	CONNECT int = 6
	OPTIONS int = 7
	TRACE   int = 8
	PATCH   int = 9
)

func min(a int, b int) int {
	if a < b {
		return a
	} else {
		return b
	}
}

func judgeRequest(data []byte, n int) int {
	str := string(data[:n])
	switch {
	case strings.HasPrefix(str, "GET"):
		return GET
	case strings.HasPrefix(str, "HEAD"):
		return HEAD
	case strings.HasPrefix(str, "POST"):
		return POST
	case strings.HasPrefix(str, "PUT"):
		return PUT
	case strings.HasPrefix(str, "DELETE"):
		return DELETE
	case strings.HasPrefix(str, "CONNECT"):
		return CONNECT
	case strings.HasPrefix(str, "OPTIONS"):
		return OPTIONS
	case strings.HasPrefix(str, "TRACE"):
		return TRACE
	case strings.HasPrefix(str, "PATCH"):
		return PATCH
	default:
		return Unknown
	}
}

func judgeResponse(data []byte, n int) int {
	if strings.HasPrefix(string(data[:n]), "HTTP") {
		return 1
	} else {
		return 0
	}
}

func parseResponse(data []byte) int64 {
	resp, err := http.ReadResponse(bufio.NewReader(bytes.NewBuffer(data)), nil)
	if err != nil {
		return -2
	}
	return resp.ContentLength
}

func decodeResponse(data []byte) ([]byte, int64) {
	str := string(data)
	ind := strings.Index(str, "\r\n\r\n") + 4
	head := make([]byte, 0)
	head = append(head, str[:ind]...)
	//index := strings.Index(str[:ind], "gzip")
	var comp bool = false
	// if index >= 0 {
	// 	comp = true
	// }
	body := make([]byte, 0)
	for {
		len := 0
		fmt.Sscanf(str[ind:], "%x", &len)
		if len == 0 {
			break
		}
		ind += strings.Index(str[ind:], "\r\n") + 2 // head
		body = append(body, str[ind:ind+len]...)
		ind += len + 2 // body
	}
	body = modifyResponseBody(body, comp)
	head = append(head, body...)
	return head, int64(len(head))
}

func Decompress(data []byte) []byte {
	buffer := bytes.Buffer{}
	fmt.Printf("byte %v\n", data[:16])
	file, _ := os.OpenFile("a.txt", os.O_CREATE|os.O_WRONLY, 0666)
	file.Write(data)
	file.Close()
	gzipReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		log.Panic(err)
		return nil
	}
	io.Copy(&buffer, gzipReader)
	gzipReader.Close()
	return buffer.Bytes()
}

func Compress(data []byte) []byte {
	buffer := bytes.Buffer{}
	gzipWriter := gzip.NewWriter(&buffer)
	gzipWriter.Write(data)
	gzipWriter.Close()
	return buffer.Bytes()
}

func modifyResponseBody(data []byte, compress bool) []byte {
	ret := make([]byte, 0)
	ret = append(ret, data...)
	if compress {
		ret = Decompress(ret)
	}
	ret = bytes.Replace(ret, []byte("百度"), []byte("搜狗"), -1)
	/*Modify there*/
	if compress {
		ret = Compress(ret)
	}
	return ret
}

func modifyResponseHead(str string, n int64) []byte {
	line := strings.Split(str, "\r\n")
	lineLen := "Content-Length: " + fmt.Sprintf("%d", n)
	for i := 0; i < len(line); i++ {
		if strings.HasPrefix(line[i], "Content-Length:") {
			line[i] = lineLen
			break
		}
		if strings.HasPrefix(line[i], "Transfer-Encoding: chunked") {
			line[i] = lineLen
			break
		}
	}
	return []byte(strings.Join(line, "\r\n"))
}

func modifyResponse(data []byte, n int64) ([]byte, int64) {
	ind := strings.Index(string(data), "\r\n\r\n")
	headstr := string(data[:ind])
	body := modifyResponseBody(data[ind:], false)
	head := modifyResponseHead(headstr, int64(len(body)-4))
	head = append(head, body...)
	return head, int64(len(head))
}

func ForwardRemote(client, remote net.Conn) {
	defer client.Close()
	defer remote.Close()
	buffer := make([]byte, 65536)
	resp := make([]byte, 0)
	Done := -1
	ind := -1
	var contentLen int64 = 0
	var Len int64 = 0
	for {
		n, err := remote.Read(buffer)
		if err != nil {
			return
		}
		if Done < 0 {
			Done = judgeResponse(buffer[:n], min(n, 8))
		}
		if Done > 0 {
			Len += int64(n)
			resp = append(resp, buffer[:n]...)
			if ind < 0 {
				ind = strings.Index(string(resp), "\r\n\r\n")
				if ind >= 0 {
					contentLen = parseResponse(resp)
					if contentLen == -2 {
						log.Panic(errors.New("parse response error"))
						return
					}
				}
			}
			if ind >= 0 {
				if contentLen < 0 { // chunk
					if strings.HasSuffix(string(resp[Len-5:Len]), "0\r\n\r\n") {
						resp, Len = decodeResponse(resp)
						Done = -1
					} else {
						continue
					}
				} else {
					if Len == int64(ind)+contentLen+4 {
						Done = -1
					} else {
						continue
					}
				}
			}
			if Done < 0 {
				resp, Len = modifyResponse(resp, Len)
				_, err = client.Write(resp[:Len])
				if err != nil {
					log.Panic(errors.New("write response error"))
					return
				}
				resp = nil
				Len = 0
				contentLen = 0
				ind = -1
			}
		} else {
			if n > 0 {
				_, err = client.Write(buffer[:n])
				if err != nil {
					log.Panic(errors.New("write response error"))
					return
				}
			}
		}
	}
}

func ForwardClient(client, remote net.Conn) {
	defer client.Close()
	defer remote.Close()
	buffer := make([]byte, 65536)
	req := make([]byte, 0)
	var Len int64 = 0
	Done := -1
	for {
		n, err := client.Read(buffer)
		if err != nil {
			return
		}
		if Done < 0 {
			Done = judgeRequest(buffer[:n], min(n, 8))
		}
		if Done > 0 {
			Len += int64(n)
			req = append(req, buffer[:n]...)
			heads := strings.Split(string(req), "\r\n")
			var flag bool = false
			for i := 0; i < len(heads); i++ {
				//fmt.Printf("head %d %s\n", i, heads[i])
				if strings.HasPrefix(heads[i], "Accept-Encoding:") {
					heads[i] = "Accept-Encoding: identity"
					flag = true
					break
				}
			}
			if !flag {
				heads[len(heads)-1] = "Accept-Encoding: identity"
				heads = append(heads, "")
			}
			if heads[len(heads)-1] == "" {
				buffer = []byte(strings.Join(heads, "\r\n"))
				n = len(buffer)
				//fmt.Printf("req %s\n", string(buffer[:n]))
				Done = -1
				req = nil
				Len = 0
			}
		}
		if n > 0 {
			_, err = remote.Write(buffer[:n])
			if err != nil {
				log.Panic(errors.New("write request error"))
				return
			}
		}
	}
}
