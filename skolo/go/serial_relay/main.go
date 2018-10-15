package main

import (
	"flag"
	"fmt"
	"io"
	"strconv"
	"sync"
	"time"

	"github.com/huin/goserial"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

// Command is an alias for a string command.
type Command string

const (
	// command understood by the Arduino to calibrate a servo.
	CMD_CALIBRATE Command = "calibrate"

	// command understood by the Arduino to reset a specific port.
	CMD_RESET Command = "reset"

	// duration how long the arduino pushes the power button.
	RESET_DELAY = 12 * time.Second

	// serial-over-USB device.
	SERIAL_DEVICE = "/dev/ttyACM0"

	// baud rate expected to by the Arduino board.
	BAUD_RATE = 9600
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
		util.Close(a.rwc)
	}
	var err error
	if a.rwc, err = goserial.OpenPort(a.conf); err != nil {
		return err
	}
	return nil
}

// retry will try to run the given function for the given number of
// times. If it fails all tries the last error will be returned.
func (a *ArduinoClient) retry(nTimes int, fn func() error) error {
	var err error
	for i := 0; i < nTimes; i++ {
		if err = fn(); (err == nil) || (err != io.EOF) {
			sklog.Errorf("Error in retry: %s", err)
			return err
		}

		if err = a.reopen(); err != nil {
			return err
		}
	}
	return err
}

// Send sends the given command and port to the Arduino board
// over the serial USB connection. It returns an error if the
// send failed.
func (a *ArduinoClient) Send(cmd Command, port int) error {
	a.mutex.Lock()

	// TODO(stephana): The retries might not be necessary. Keeping it
	// for stability right now, but should be removed if we don't see
	// errors in production. Errors are logged in the retry function.
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

func (a *ArduinoClient) Close() error {
	return a.rwc.Close()
}

func main() {
	common.Init()
	ports := flag.Args()

	client, err := NewArduinoClient(SERIAL_DEVICE, BAUD_RATE)
	if err != nil {
		sklog.Fatalf("Error: %s", err)
	}
	defer util.Close(client)

	if err := client.Send(CMD_CALIBRATE, 1); err != nil {
		sklog.Fatalf("Error callibrating: %s", err)
	}

	for _, portStr := range ports {
		port, err := strconv.ParseInt(portStr, 10, 64)
		if err != nil {
			sklog.Fatalf("Wrong port %s. Needs to be an integer. ", portStr)
		}

		if err := client.Send(CMD_RESET, int(port)); err != nil {
			sklog.Fatalf("Error writing: %s", err)
		}
	}
}
