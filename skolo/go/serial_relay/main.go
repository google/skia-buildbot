package main

import (
	"bytes"
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
	RESET_DELAY           = 3 * time.Second
)

type ArduinoClient struct {
	conf  *goserial.Config
	rwc   io.ReadWriteCloser
	buf   []byte
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

func (a *ArduinoClient) retry(nTimes int, fn func() error) error {
	var err error
	for i := 0; i < nTimes; i++ {
		if err = fn(); (err == nil) || (err != io.EOF) {
			return err
		}

		if err = a.reopen(); err != nil {
			return err
		}
	}
	return err
}

func (a *ArduinoClient) Send(cmd Command, port int) error {
	a.mutex.Lock()
	err := a.retry(3, func() error {
		_, err := a.rwc.Write([]byte(fmt.Sprintf("%s %d", cmd, port)))
		return err
	})
	a.mutex.Unlock()

	if err != nil {
		return err
	}
	time.Sleep(RESET_DELAY)
	return err
}

func (a *ArduinoClient) ReadLine() (string, error) {
	readBuf := make([]byte, 256)
	for {
		if len(a.buf) > 0 {
			if idx := bytes.Index(a.buf, []byte("\n")); idx >= 0 {
				ret := string(append([]byte(nil), a.buf[:idx]...))
				newBuf := a.buf[:len(a.buf)-idx-1]
				copy(newBuf, a.buf[idx+1:])
				a.buf = newBuf
				return ret, nil
			}
		}

		var n int
		a.mutex.Lock()
		err := a.retry(3, func() error {
			var err error
			n, err = a.rwc.Read(readBuf)
			return err
		})
		a.mutex.Unlock()
		if err != nil {
			return "", err
		}

		if n > 0 {
			a.buf = append(a.buf, readBuf[:n]...)
		} else {
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// func (a *ArduinoClient) ReadLine() (string, error) {
// 	a.mutex.Lock()
// 	defer a.mutex.Unlock()
// 	r := bufio.NewReader(a.rwc)
// 	ret, err := r.ReadString('\n')
// 	if err != nil {
// 		return "", err
// 	}
// 	return ret, nil
// }

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
	client, err := NewArduinoClient("/dev/ttyACM1", 9600)
	if err != nil {
		log.Fatalf("Error: %s", err)
	}
	defer client.Close()

	done := false
	go func() {
		for !done {
			msg, err := client.ReadLine()
			if err != nil {
				log.Fatalf("Error while reading: %s", err)
			}
			fmt.Println(msg)
		}
	}()
	time.Sleep(time.Second)

	if err := client.Send(CMD_CALIBRATE, 1); err != nil {
		log.Fatalf("Error callibrating: %s", err)
	}

	for port := 1; port <= 6; port++ {
		if err := client.Send(CMD_RESET, port); err != nil {
			log.Fatalf("Error writing: %s", err)
		}
	}
	done = true
	time.Sleep(time.Second)
}
