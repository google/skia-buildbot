package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/huin/goserial"
)

type Command string

const (
	CMD_CALIBRATE Command = "calibrate"
	CMD_RESET     Command = "reset"
)

type ArduinoClient struct {
	conf  *goserial.Config
	rwc   io.ReadWriteCloser
	mutex sync.RWMutex
}

func NewArduinoClient(devName string, baud int) (*ArduinoClient, error) {
	ret := &ArduinoClient{
		conf: &goserial.Config{Name: devName, Baud: baud},
	}
	if err := ret.reopen(); err != nil {
		return nil, err
	}
	return ret, nil
}

func (a *ArduinoClient) reopen() error {
	if a.rwc != nil {
		_ = a.rwc.Close()
	}
	var err error
	if a.rwc, err = goserial.OpenPort(a.conf); err != nil {
		return err
	}
	return nil
}

func (a *ArduinoClient) Send(cmd Command, port int) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	_, err := a.rwc.Write([]byte(fmt.Sprintf("%s %d", cmd, port)))
	if err != nil {
		return err
	}
	time.Sleep(time.Second * 15)
	return err
}

// func (a *ArduinoClient) ReadLine() (string, error) {
// 	readBuf := make([]byte, 256)
// 	for {
// 		if len(a.buf) > 0 {
// 			if idx := bytes.Index(a.buf, []byte("\n")); idx >= 0 {
// 				ret := string(append([]byte(nil), a.buf[:idx]))
// 				copy(a.)
// 				return ret
// 			}
// 		}

// 		n, err := a.rwc.Read(readBuf)
// 		if err != nil {
// 			return "", err
// 		}
// 		if n == 0 {
// 			time.Sleep(10 * time.Millisecond)
// 		} else {
// 			a.buf = append(a.buf, readBuf[:n])
// 		}
// 	}
// func main() {
// 	arr := make([]byte, 0, 250)
// 	arr = append(arr, []byte("hello\nworld")...)
// 	fmt.Println(arr)
// 	fmt.Printf("%d %d\n", len(arr[2:]), cap(arr[2:]))
// 	idx := bytes.Index(arr, []byte("\n"))
// 	ret := string(append([]byte(nil), arr[:idx]...))
// 	fmt.Printf("RET: %s\n", ret)
// 	newArr := arr[:len(arr)-idx-1]
// 	copy(newArr, arr[idx+1:])
// 	fmt.Printf("%s %d %d\n", string(arr), len(arr), cap(arr))
// 	fmt.Printf("%s %d %d\n", newArr, len(newArr), cap(newArr))
// }
// }

func (a *ArduinoClient) ReadLine() (string, error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	r := bufio.NewReader(a.rwc)
	ret, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	return ret, nil
}

// func (a *ArduinoClient) execCmd(cmd Command, arg string) error {
// 	msg := fmt.Sprintf("%s %s\n", cmd, arg)

// 	if _, err := a.rwc.Write([]byte(msg)); err != nil {
// 		return fmt.Errorf("Error writing: %s", err)
// 	}

// 	fmt.Printf("command %s written\n", cmd)
// 	//resp, err := a.bufReader.ReadString('\n');
// 	//if err != nil {
// 	//return err
// 	//}

// 	//if strings.HasPrefix(resp, "ERR ") {
// 	//return fmt.Errorf("Error: %s", resp[4:])
// 	//}
// 	//fmt.Printf("Received: %s\n", resp)
// 	return nil
// }

func (a *ArduinoClient) Close() error {
	return a.rwc.Close()
}

func main() {
	client, err := NewArduinoClient("/dev/ttyACM0", 9600)
	if err != nil {
		log.Fatalf("Error: %s", err)
	}
	defer client.Close()

	// go func() {
	// 	for {
	// 		msg, err := client.ReadLine()
	// 		if err != nil {
	// 			log.Fatalf("Error while reading: %s", err)
	// 		}
	// 		fmt.Println(msg)
	// 	}
	// }()

	for port := 1; port <= 2; port++ {
		fmt.Printf("Resetting port %d.", port)
		if err := client.Send(CMD_RESET, port); err != nil {
			log.Fatalf("Error writing: %s", err)
		}
		fmt.Println("Done.")
	}
}
