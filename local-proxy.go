package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
)

type connectHandler struct {
	LAddr net.TCPAddr
}

func read(r *bufio.Reader, c chan []byte, errc chan error) {
	for {
		buf := make([]byte, 4096)
		n, err := r.Read(buf)
		if err != nil {
			errc <- err
			return
		}
		c <- buf[0:n]
	}
}

func (h connectHandler) proxyConnection(clientConn net.Conn, clientRw *bufio.ReadWriter, hostName string) {
	defer clientConn.Close()

	raddr, err := net.ResolveTCPAddr("tcp4", hostName)
	if err != nil {
		log.Println("Error resolving host:", err)
		return
	}

	serverConn, err := net.DialTCP("tcp4", &h.LAddr, raddr)
	if err != nil {
		log.Println("Failed to connect to", hostName, "from", h.LAddr, ":", err)
		return
	}
	defer serverConn.Close()
	log.Println("Connected to", hostName)

	clientReadChan := make(chan []byte)
	clientErrChan := make(chan error)
	serverReadChan := make(chan []byte)
	serverErrChan := make(chan error)
	serverReader := bufio.NewReader(serverConn)
	serverWriter := bufio.NewWriter(serverConn)

	go read(clientRw.Reader, clientReadChan, clientErrChan)
	go read(serverReader, serverReadChan, serverErrChan)

	for {
		select {
		case line := <-clientReadChan:
			_, err := serverWriter.Write(line)
			if err != nil {
				log.Println("Error writing to server:", err)
				return
			}
			serverWriter.Flush()
			log.Println("client->server:", len(line), "bytes")

		case clientErr := <-clientErrChan:
			log.Println("Error reading from client:", clientErr)
			return

		case line := <-serverReadChan:
			_, err := clientRw.Writer.Write(line)
			if err != nil {
				log.Println("Error writing to client:", err)
				return
			}
			clientRw.Writer.Flush()
			log.Println("server->client:", len(line), "bytes")

		case serverErr := <-serverErrChan:
			log.Println("Error reading from server:", serverErr)
			return
		}
	}
}

func (h connectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "CONNECT" {
		log.Println("Got a non-connect request:", r)
		http.Error(w, "This server only supports CONNECT", http.StatusNotImplemented)
		return
	}

	w.WriteHeader(http.StatusOK)

	hj, ok := w.(http.Hijacker)
	if !ok {
		log.Println("Error hijacking connection")
		http.Error(w, "Couldn't create hijacker", http.StatusInternalServerError)
		return
	}
	conn, rwbuf, err := hj.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Println("Successfully hijacked")

	hostName := r.URL.Host
	log.Println("Desired destination:", r.URL.Host)

	go h.proxyConnection(conn, rwbuf, hostName)
}

func getLocalTCP4Addr(iface *net.Interface) (net.TCPAddr, error) {
	addrs, err := iface.Addrs()
	if err != nil {
		log.Panicln("Error getting interface addrs:", err)
	}
	for _, addr := range addrs {
		ip, _, err := net.ParseCIDR(addr.String())
		if err != nil {
			continue
		}
		if ip.IsGlobalUnicast() {
			return net.TCPAddr{IP: ip}, nil
		}
	}

	return net.TCPAddr{}, errors.New("No good address")
}

func printRoutingLines(iface *net.Interface) {
	addrs, err := iface.Addrs()
	if err != nil {
		log.Panicln("Error getting interface addrs:", err)
	}
	for _, addr := range addrs {
		ip, ipnet, err := net.ParseCIDR(addr.String())
		if err != nil {
			continue
		}
		if !ip.IsGlobalUnicast() {
			continue
		}

		baseIpBytes := []byte(ipnet.IP)
		// The gateway is just a guess, of course: the first IP in the network
		// whose last octet isn't 0.
		gateway := net.IPv4(baseIpBytes[0], baseIpBytes[1], baseIpBytes[2], baseIpBytes[3]+1)
		fmt.Println("==================== CONFIG INFORMATION ====================")
		fmt.Println("sudo ip route flush table rt2")
		fmt.Printf("sudo ip route add %v dev %v proto kernel src %v table main\n", ipnet, iface.Name, ip)
		fmt.Printf("sudo ip route add default via %v dev %v table rt2\n", gateway.String(), iface.Name)
		fmt.Printf("sudo ip rule add from %v/32 table rt2\n", ip)
		fmt.Printf("sudo ip rule add to %v/32 table rt2\n", ip)
		fmt.Println("============================================================")
	}
}

func main() {
	if len(os.Args) < 2 {
		log.Panicln("Usage:", os.Args[0], "<iface>")
	}
	ifaceName := os.Args[1]
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		log.Panicln("Error finding interface:", err)
	}
	ip, err := getLocalTCP4Addr(iface)
	if err != nil {
		log.Panicln(err)
	}

	printRoutingLines(iface)

	http.ListenAndServe("127.0.0.1:8080", connectHandler{LAddr: ip})
}
