package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func main() {
	cmd := exec.Command("cockroach", "start-single-node", "--insecure")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGKILL,
	}

	err := cmd.Start()
	if err != nil {
		panic(err)
	}

	go func() {
		err := cmd.Wait()
		if err != nil {
			fmt.Printf("ERROR WAITING FOR COMMAND: %v\n", err)
			panic(err)
		}
		ws := cmd.ProcessState.Sys().(syscall.WaitStatus)
		exitCode := ws.ExitStatus()
		fmt.Printf("FINISHED WITH EXIT CODE: %d\n", exitCode)
	}()

	fmt.Println("SLEEPING FOR 10 SECONDS")
	time.Sleep(10 * time.Second)
	fmt.Println("EXITING")
}
