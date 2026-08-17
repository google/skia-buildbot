#include <unistd.h>
#include <stdio.h>
#include <sys/socket.h>
#include <stdlib.h>
#include <netinet/in.h>

void draw(void *canvas) {
   int server_fd;
   if ((server_fd = socket(AF_INET, SOCK_STREAM, 0)) == 0) {
       perror("socket failed");
       exit(EXIT_FAILURE);
   }
}
