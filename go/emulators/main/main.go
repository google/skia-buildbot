package main

/*
#include <stdio.h>
#include <signal.h>
#include <unistd.h>
#include <stdlib.h>
#include <errno.h>
#include <string.h>

static int int_count = 0, max_int = 5;
static struct sigaction siga;

static void multi_handler(int sig, siginfo_t *siginfo, void *context) {
    // get pid of sender,
    pid_t sender_pid = siginfo->si_pid;

    printf("SIGNAL RECEIVED: %d, SENDER PID: %d\n", sig, (int) sender_pid);

    return;
}

int raise_test() {
    // print pid
    printf("process [%d] started.\n", (int)getpid());

    // prepare sigaction
    siga.sa_sigaction = *multi_handler;
    siga.sa_flags |= SA_SIGINFO; // get detail info

    // change signal action,
    if(sigaction(SIGINT, &siga, NULL) != 0) {
        printf("error sigaction() for signal %d\n", SIGINT);
        return errno;
    }
    if(sigaction(SIGQUIT, &siga, NULL) != 0) {
        printf("error sigaction() for signal %d\n", SIGQUIT);
        return errno;
    }
    if(sigaction(SIGTERM, &siga, NULL) != 0) {
        printf("error sigaction() for signal %d\n", SIGTERM);
        return errno;
    }

    return 0;
}
*/
import "C"
import (
	"fmt"
	"time"
)

func main() {
	//C.sigprint(C.int(syscall.SIGTERM))

	C.raise_test()

	fmt.Println("SLEEPING")
	time.Sleep(1 * time.Hour)

	//cmd := exec.Command("cockroach", "start-single-node", "--insecure")
	//cmd.Stdout = os.Stdout
	//cmd.Stderr = os.Stderr
	//cmd.SysProcAttr = &syscall.SysProcAttr{
	//	Pdeathsig: syscall.SIGKILL,
	//}
	//
	//err := cmd.Start()
	//if err != nil {
	//	panic(err)
	//}
	//
	//go func() {
	//	err := cmd.Wait()
	//	if err != nil {
	//		fmt.Printf("ERROR WAITING FOR COMMAND: %v\n", err)
	//		panic(err)
	//	}
	//	ws := cmd.ProcessState.Sys().(syscall.WaitStatus)
	//	exitCode := ws.ExitStatus()
	//	fmt.Printf("FINISHED WITH EXIT CODE: %d\n", exitCode)
	//}()
	//
	//fmt.Println("SLEEPING FOR 10 SECONDS")
	//time.Sleep(10 * time.Second)
	//fmt.Println("EXITING")
}
