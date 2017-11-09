package http

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"time"
)

type Handler func(*Response, *Request)

type Response struct {
	Status int

	keepalive bool
	buf       bytes.Buffer
}

type Request struct {
	Method string
	URI    string

	Headers bytes.Buffer
	Body    io.Reader
}

type Server struct {
	Addr     string  // TCP address to listen on, ":http" if empty
	Handler  Handler // handler to invoke, http.DefaultServeMux if nil
	ErrorLog *log.Logger

	listener net.Listener
}

type httpConn struct {
	netConn net.Conn
	handler Handler
}

var (
	bad       = []byte("HTTP/1.1 400 Bad Request\r\nConnection: close\r\n\r\n")
	keepalive = []byte("keep-alive")
)

func New(addr string) *Server {
	return &Server{
		Addr: addr,
	}
}

func (srv *Server) ListenAndServe() error {
	var err error
	srv.listener, err = net.Listen("tcp", srv.Addr)
	if err != nil {
		return err
	}
	defer srv.listener.Close()

	for {
		nc, err := srv.listener.Accept()
		if err != nil {
			return err
		}

		con := httpConn{nc, srv.Handler}

		// Spawn off a goroutine so we can accept other connections.
		go con.serve()
	}
	return nil
}

func (hc *httpConn) serve() {
	defer hc.netConn.Close()

	buf := bufio.NewReader(hc.netConn)

	for {
		req, err := readRequest(buf)
		if err != nil {
			hc.netConn.Write(bad)
			return
		}

		res := Response{
			Status: 200,
		}

		res.keepalive = req.isKeepalive()
		//if echo {
		//	res.Headers.WriteString("Connection = keep-alive")
		//}

		hc.handler(&res, req)

		if err := res.writeTo(hc.netConn); err != nil {
			return
		}

		if !res.keepalive {
			return
		}
	}
}

func (req *Request) isKeepalive() bool {
	if bytes.Contains(req.Headers.Bytes(), keepalive) {
		return true
	}
	return false
}

func readRequest(buf *bufio.Reader) (*Request, error) {
	req := &Request{}

	if ln0, err := readHTTPLine(buf); err == nil {
		var ok bool
		if req.Method, req.URI, ok = parseRequestLine(ln0); !ok {
			return nil, fmt.Errorf("malformed request line: %q", ln0)
		}
	}

	// Read each subsequent header.
	for {
		ln, err := readHTTPLine(buf)
		if err != nil {
			return nil, err
		}

		if len(ln) == 0 {
			break
		}

		req.Headers.WriteString(ln)
		//if key, val, ok := parseHeaderLine(ln); ok {
		//	req.Headers[key] = val
		//}
	}

	return req, nil
}

func readHTTPLine(buf *bufio.Reader) (string, error) {
	ln, err := buf.ReadString('\n')
	if err != nil {
		return "", err
	}

	return strings.TrimSuffix(ln, "\r\n"), nil
}

func parseRequestLine(ln string) (method, uri string, ok bool) {
	s := strings.Split(ln, " ")
	if len(s) != 3 {
		return
	}

	return s[0], s[1], true
}

func parseHeaderLine(ln string) (key, val string, ok bool) {
	s := strings.SplitN(ln, ":", 2)
	if len(s) != 2 {
		return
	}

	return strings.ToLower(s[0]), strings.TrimSpace(s[1]), true
}

func (res *Response) writeTo(w io.Writer) error {
	if err := res.writeHeadersTo(w); err != nil {
		return err
	}

	if _, err := res.buf.WriteTo(w); err != nil {
		return err
	}

	return nil
}

func (res *Response) writeHeadersTo(w io.Writer) error {
	statusText := ""
	switch res.Status {
	case 200:
		statusText = "OK"
	case 201:
		statusText = "Created"
	case 202:
		statusText = "Accepted"
	case 203:
		statusText = "Non-Authoritative Information"
	case 204:
		statusText = "No Content"
		// TODO: More status codes
	}

	if statusText == "" {
		return fmt.Errorf("unsupported status code: %v", res.Status)
	}

	// https://www.w3.org/Protocols/rfc2616/rfc2616-sec6.html
	headers := fmt.Sprintf("%s %v %s\r\n", "1.1", res.Status, statusText)
	headers += fmt.Sprintf("Date: %s\r\n", time.Now().UTC().Format("Mon, 02 Jan 2006 15:04:05 GMT"))
	if res.keepalive {
		headers += fmt.Sprintf("Connection: %s\r\n", string(keepalive))
	}
	headers += fmt.Sprintf("Content-Length: %d\r\n", res.buf.Len())
	headers += "\r\n"

	if _, err := w.Write([]byte(headers)); err != nil {
		return err
	}

	return nil
}
