package client

import (
	"bufio"
	"flag"
	"fmt"
	"net"
	"os"
)

var (
	BUFFER_SIZE = 1024
	MTCP_EOF    = "MTCP_EOF"
	CLIENT_S    = "CLIENT_S"
	ip          string
	port        string
	server      string
)

type Client struct {
	conn       net.Conn
	state_type string
}

func init() {
	// 定义命令行标志
	// 设置标志的默认值和帮助信息
	flag.StringVar(&ip, "ip", "127.0.0.1", "代理ip")
	flag.StringVar(&port, "port", "80", "代理端口")
	flag.StringVar(&server, "server", "127.0.0.1:9999", "中心转发服务")

	// 解析命令行标志
	flag.Parse()
}

func Main() {
	for i := 0; i < 9; i++ {
		go copyAA()
	}
	copyAA()
}

func copyAA() {
	var localBase *Client
	c := make(chan []byte)
	tcpAddr, err := net.ResolveTCPAddr("tcp", server)
	if err != nil {
		println("ResolveTCPAddr failed:", err.Error())
		os.Exit(1)
	}

	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		println("Dial failed:", err.Error())
		os.Exit(1)
	}
	conn.Write([]byte(CLIENT_S))
	go readServerMsg(conn, c)

	for {
		select {
		case data := <-c:
			if localBase == nil || localBase.state_type == "EOF" {
				conn1, err := net.Dial("tcp", ip+":"+port)
				if err != nil {
					fmt.Println("连接错误:", err)
				}
				defer conn1.Close()
				client := &Client{
					conn: conn1,
				}
				localBase = client
				go readLocalMsg(conn, client)
			}
			localBase.conn.Write(data)
		}
	}
}

// 读取本地发来数据
func readLocalMsg(conn *net.TCPConn, localBase *Client) {

	reader := bufio.NewReader(localBase.conn)
	for {
		reply := make([]byte, BUFFER_SIZE)

		n, err := reader.Read(reply)
		if err != nil {
			println("readLocalMsg Write to server failed:", err.Error())
			conn.Write([]byte(MTCP_EOF))
			localBase.conn.Close()
			localBase.state_type = "EOF"
			break
		}
		//fmt.Printf(string(reply[:n]))
		conn.Write(reply[:n])
	}
	fmt.Printf("本地服务的结束响应 \n")
}

// 服务器发来代理数据
func readServerMsg(conn *net.TCPConn, c chan []byte) {
	reader := bufio.NewReader(conn)
	for {
		reply := make([]byte, BUFFER_SIZE)

		n, err := reader.Read(reply)
		if err != nil {
			println("readServerMsg Write to server failed:", err.Error())
			os.Exit(1)
		}
		// 给channel发信号
		c <- reply[:n]
	}
}
