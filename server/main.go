package server

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/google/uuid"
	"log"
	"net"
	"strings"
	"sync"
)

var (
	browserClientMap = make(map[string]*Client)
	mu               sync.Mutex // 用于保护 browserClientMap 的访问
	BUFFER_SIZE      = 1024
	MTCP_EOF         = "MTCP_EOF"
	CLIENT_S         = "CLIENT_S"
	POOL_SIZE        = 10
	CLOSE_SIZE       = 0
	POOL             = newClientPool(POOL_SIZE)
	port             string
)

func init() {
	// 定义命令行标志
	// 设置标志的默认值和帮助信息
	flag.StringVar(&port, "port", "9999", "监听端口")

	// 解析命令行标志
	flag.Parse()
}

type Client struct {
	conn        net.Conn
	id          string
	network     string
	localClient *Client
}

// clientPool 管理 localClient 的池
type clientPool struct {
	localClient chan *Client
	mu          sync.Mutex
}

// newClientPool 创建一个新的 clientPool
func newClientPool(size int) *clientPool {
	return &clientPool{
		localClient: make(chan *Client, size),
	}
}

// addClient 向池中添加一个空闲的 localClient
func (p *clientPool) addClient(client *Client) {
	p.localClient <- client
}

// getFreeClient 获取一个空闲的 localClient，如果没有则等待
func (p *clientPool) getFreeClient() *Client {
	client := <-p.localClient
	log.Printf("拿到了可用的 %s \n", client.id)
	return client
}

// releaseClient 释放一个 localClient 回到池中
func (p *clientPool) releaseClient(client *Client) {
	p.addClient(client)
	log.Printf("释放用完的 %s \n", client.id)
}

func Main() {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%s", "0.0.0.0", port))
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()
	fmt.Printf("服务队启动。。。")

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal(err)
		}
		newUUID, _ := uuid.NewRandom()

		client := &Client{
			conn:    conn,
			id:      strings.ReplaceAll(newUUID.String(), "-", ""),
			network: "tcp",
		}
		// 区分http请求，还是客户端连接
		go client.handleRequest()
	}
}

// 处理所有数据
func (client *Client) handleRequest() {
	reader := bufio.NewReader(client.conn)
	for {
		// 创建一个缓冲区来读取数据
		buffer := make([]byte, BUFFER_SIZE)
		n, err := reader.Read(buffer)
		if err != nil {
			client.conn.Close()
			fmt.Printf("关闭了客户端: %s 类型： %s\n", client.id, client.network)
			if client.network == "browser" {
				POOL.releaseClient(client.localClient)
			}
			if client.network == "local" {
				CLOSE_SIZE++
				if CLOSE_SIZE == POOL_SIZE {
					clearChannel(POOL.localClient)
				}
			}
			break
		}
		flag := string(buffer[:8])
		if client.network == "tcp" {
			if flag == CLIENT_S {
				client.network = "local"
				POOL.addClient(client)
				fmt.Printf("添加了local TCP连接 %s \n", client.id)
				continue
			} else {
				client.network = "browser"
				mu.Lock()
				browserClientMap[client.id] = client
				mu.Unlock()
				fmt.Printf("添加了浏览器HTTP连接 %s \n", client.id)
			}
		}
		// 打印收到的数据
		//fmt.Printf("Received %d bytes: %s\n", n, buffer[:n])
		if client.network == "browser" {
			if client.localClient == nil {
				// 查找一个空闲localClient，没有就等待
				localClient := POOL.getFreeClient()
				// 建立关联
				client.localClient = localClient
				localClient.localClient = client
			}
			client.localClient.conn.Write(buffer[:n])
		}
		if client.network == "local" {
			if flag == MTCP_EOF {
				fmt.Printf("local的TCP主动关闭连接yes \n")
				client.localClient.conn.Close()
				mu.Lock()
				delete(browserClientMap, client.localClient.id)
				mu.Unlock()
				client.localClient = nil
			} else {
				client.localClient.conn.Write(buffer[:n])
			}

		}
	}
	fmt.Printf("浏览器服务的结束响应\n")
}

func clearChannel(ch <-chan *Client) {
	fmt.Println("clearChannel")
	for range ch {
		// 读取并丢弃数据
		CLOSE_SIZE--
		if CLOSE_SIZE == 0 {
			break
		}
	}
	fmt.Println("clearChannelEnd")
}
