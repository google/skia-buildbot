
#include <Bridge.h>
#include <BridgeServer.h>
#include <BridgeClient.h>
#include <Wire.h>
#include <Adafruit_PWMServoDriver.h>

// These values are arbitrarily picked, because they fall within the range
// of most servos (150 - 600).  The delta of 30 units was determined
// experimentally.
#define UP 200
#define DOWN 175

// called this way, it uses the default address 0x40
Adafruit_PWMServoDriver pwm = Adafruit_PWMServoDriver();

// Listens to the localhost:5555
// However, the Yún webserver will forward all the
// HTTP requests it recieves to that port, so we don't really
// care what port it runs on.
BridgeServer server;

// This code brings up a webserver that listens for reboot commands
// as a "REST" API.
// For example, performing a GET request to
// http://[ip_address]/arduino/reboot/0
// Will issue the servo on port 0 to be depressed for a bit and then released.
// That request blocks until the servo is released.

// If the Seeduino itself is powercycled, it can take up to 45 seconds
// to turn on, get connected to the network and start the web server.


void setup() {
  // Bridge takes about two seconds to start up
  // it can be helpful to use the on-board LED
  // as an indicator for when it has initialized
  pinMode(LED_BUILTIN, OUTPUT);
  digitalWrite(LED_BUILTIN, LOW);
  Bridge.begin();
  digitalWrite(LED_BUILTIN, HIGH);

  // Boilerplate to turn on
  Serial.begin(9600);

  pwm.begin();

  pwm.setPWMFreq(60);  // Analog servos run at ~60 Hz updates

  for (uint8_t i = 0; i < 16; i++) {
    pwm.setPWM(i, 0, UP);
  }

  // Listen for incoming connection only from localhost
  // (no one from the external network could connect)
  server.listenOnLocalhost();
  server.begin();
}

void loop() {
  // Get clients coming from server
  BridgeClient client = server.accept();

  // There is a new client?
  if (client) {
    // Process request
    process(client);

    // Close connection and free resources.
    client.stop();
  }

  delay(50); // Poll every 50ms
}

void process(BridgeClient client) {
  // read the command
  String command = client.readStringUntil('/');

  // is "digital" command?
  if (command == "reboot") {
    rebootCommand(client);
  }

}

void rebootCommand(BridgeClient client) {
  // Read pin number
  int servo = client.parseInt();

  if (servo < 0) {
    servo = 0;
  }
  if (servo > 15) {
    servo = 15;
  }

  client.print("Rebooted port ");
  client.print(servo);

  pwm.setPWM(servo, 0, DOWN);

  delay(12000);

  pwm.setPWM(servo, 0, UP);

  // For reasons unknown, responses are cut short
  // if they take longer than about 5 seconds.
}

