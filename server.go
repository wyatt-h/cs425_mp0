package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
	"sync"
)

const (
	CONN	= "%f - %s connected\n"
	DISCONN	= "%f - %s disconnected\n"
	EVENT	= "%s %s %s\n"
	BW_FILE	= "bandwidths.txt"
	DL_FILE	= "delays.txt"
	CLEAR	= 0
	ADD		= 1
)

var (
	dllock		sync.Mutex
	bwlock		sync.Mutex
	bandwidths	int
	delays = 	make([]float64, 0)
)

func handleConnection(c net.Conn, connTime float64) {
	var node string

	for {
		data, err := bufio.NewReader(c).ReadString('\n')
		if err != nil {
			if err == io.EOF {
				fmt.Printf(DISCONN, getCurrentTime(), node)
				break
			}

			fmt.Println(err)
			break
		}

		req := strings.Fields(data)

		if len(req) == 1 {
			// node just connected
			node = req[0]
			fmt.Printf(CONN, connTime, node) 

			// measure bandwidth
			updateBandwidths(ADD, len(node))
		
		} else if len(req) == 2 {
			// measure delay
			eventTime, err := strconv.ParseFloat(req[0], 64)
			if (err != nil) {
				fmt.Println(err)
			}

			updateDelays(ADD, getCurrentTime() - eventTime)

			// print event
			fmt.Printf(EVENT, req[0], node, req[1])

			// measure bandwidth
			updateBandwidths(ADD, len(req[1]))
		}
	}
	c.Close()
}

func saveMeasurement(fileName string, data string) {
	f, err := os.OpenFile(fileName, os.O_WRONLY | os.O_CREATE | os.O_APPEND, 0755)
	if err != nil {
		fmt.Println(err)
		return
	}

	defer f.Close()
	
	_, err = f.WriteString(data + "\n")
	if err != nil {
		fmt.Println(err)
	}
}


func getCurrentTime() float64 {
	return float64(time.Now().UnixNano()) / 1_000_000_000.0 
}

func updateBandwidths(opcode int, val int) {
	bwlock.Lock()
	defer bwlock.Unlock()

	switch opcode {
	case CLEAR:
		bandwidths = 0
	case ADD:
		bandwidths += val
	default:
		return
	}
}

func updateDelays(opcode int, val float64) {
	dllock.Lock()
	defer dllock.Unlock()

	switch opcode {
	case CLEAR:
		delays = delays[:0]
	case ADD:
		delays = append(delays, val)
	default:
		return
	}
}

func trackMetrics() {
	ticker := time.NewTicker(time.Second) 

	defer ticker.Stop()

	for {
		select {
		case <- ticker.C:
			saveMeasurement(BW_FILE, strconv.Itoa(bandwidths))

			if len(delays) == 0 {
				saveMeasurement(DL_FILE, "0")
			} else {
				s := make([]string, 0)
				for _,d := range delays {
					s = append(s, strconv.FormatFloat(d, 'f', -1, 64))
				}
				saveMeasurement(DL_FILE, strings.Join(s, " "))
			}

			updateDelays(CLEAR, 0)
			updateBandwidths(CLEAR, 0)
		}
	}
}

func main() {
	arguments := os.Args
	if len(arguments) == 1 {
		fmt.Println("Please specify port")
	}

	PORT := ":" + arguments[1]
	l, err := net.Listen("tcp4", PORT)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer l.Close()

	go trackMetrics()

	for {
		c, err := l.Accept()
		if err != nil {
			fmt.Println(err)
			return
		}

		go handleConnection(c, getCurrentTime())
	}
}

