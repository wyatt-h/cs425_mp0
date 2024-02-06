package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"reflect"
)

const (
	CONN	= "%f - %s connected\n"
	DISCONN	= "%f - %s disconnected\n"
	EVENT	= "%s %s %s\n"
	BW_FILE	= "bandwidths.txt"
	DL_FILE	= "delays.txt"
	ADD     = 0
	REPLACE = 1
)

var (
	dlLock				sync.Mutex
	bandwidths			int64
	delays				= make([]float64, 0)
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
			atomic.AddInt64(&bandwidths, int64(len(node)))
		
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
			atomic.AddInt64(&bandwidths, int64(len(req[1])))
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

func compareAndSwapDelays(oldS []float64, newS []float64) bool {
	dlLock.Lock()
	defer dlLock.Unlock()
	
	if reflect.DeepEqual(delays, oldS) {
		delays = newS
		return true
	}
	return false
}

func updateDelays(opCode int, vals ...float64) {
	dlLock.Lock()
	defer dlLock.Unlock()

	switch opCode {
	case ADD:
		delays = append(delays, vals...)
	case REPLACE:
		delays = vals
	default:
		return
	}
}

func copyDelays(s []float64) []float64 {
	dlLock.Lock()
	defer dlLock.Unlock()

	newS := make([]float64, len(s))
	copy(newS, s)
	return newS
}

func trackMetrics() {
	ticker := time.NewTicker(time.Second) 

	defer ticker.Stop()

	for {
		select {
		case <- ticker.C:
			oldBandwidths := atomic.LoadInt64(&bandwidths)
			oldDelays := copyDelays(delays)

			saveMeasurement(BW_FILE, strconv.FormatInt(oldBandwidths, 10))

			if len(oldDelays) == 0 {
				saveMeasurement(DL_FILE, "0")
			} else {
				s := make([]string, 0)
				for _,d := range oldDelays {
					s = append(s, strconv.FormatFloat(d, 'f', -1, 64))
				}
				saveMeasurement(DL_FILE, strings.Join(s, " "))
			}
			
			if !atomic.CompareAndSwapInt64(&bandwidths, oldBandwidths, 0) {
				atomic.AddInt64(&bandwidths, -oldBandwidths)
			}

			if !compareAndSwapDelays(oldDelays, make([]float64, 0)) {
				updateDelays(REPLACE, delays[len(oldDelays) + 1:]...)
			}
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


