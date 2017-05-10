#include <Servo.h>

// ServoPin combines a Servo object with the
// pin it is connected to. This must be matched
// by the physical hardware setup.
typedef struct {
  Servo servo;
  int   pin;
} ServoPin;

// servos connects the servo index to a Servo object and
// the pin on the board the servo is connected to.
ServoPin servos[] = {
  { Servo(), 3},     // servo 1
  { Servo(), 5},     // servo 2 ...
  { Servo(), 6},
  { Servo(), 9},
  { Servo(), 10},
  { Servo(), 11}
};

// N_SERVOS is used to iterate over servos and validate
// provided servo indices.
const int N_SERVOS = sizeof(servos)/sizeof(ServoPin);

// Onboard LED used to signal that there was an error.
// It blinks to indicate error. Only really used during debugging.
const int LED_PIN = 13;

// OP_CALIBRATE command puts the servo into a defined position.
// This defined position allows the lever to be added to the servo
// in a well-known position such that it can activate the power switch.
const char* OP_CALIBRATE = "calibrate";

// OP_RESET runs through the reset routine.
const char* OP_RESET     = "reset";

// Position of the servo at which the lever hovers over the power button.
const int HOVER_POSITION = 20;

// Position of the servo at which the lever pushes onto the power button.
const int PUSH_POSITION = 10;

// Duration during which the servo pushes on the power button during reset.
const int RESET_DELAY = 12000;

void setup(){
  pinMode(LED_PIN, OUTPUT);
  digitalWrite(LED_PIN, LOW);

  // Initialize the serial interface to run at 9600 baud.
  Serial.begin(9600);
  for(int i=0; i < N_SERVOS; i++) {
    servos[i].servo.attach(servos[i].pin);
    calibrateServo(i);
  }
}

boolean validPositiveInt(const String& intStr, int& result) {
  if (intStr.length() == 0) {
    return false;
  }
  for(int i=0; i<intStr.length(); i++) {
    if (!isDigit(intStr[i])) {
      return false;
    }
  }

  result = intStr.toInt();
  return true;
}

void loop(){
  if (Serial.available())  {
     String cmd, arg;
     readCommand(&cmd, &arg);

     if (cmd == "") {
       return;
     }

     int intArg;
     if (!validPositiveInt(arg, intArg) || (intArg < 1) ||
         (intArg > N_SERVOS)) {
       blinkLED(10);
       return;
     }

     if (cmd == OP_CALIBRATE) {
       calibrateServo(intArg-1);
     } else if (cmd == OP_RESET) {
       resetServo(intArg-1);
     } else {
       blinkLED(3);
     }
  }
  delay(100);
}

// Calibrate the servo.
void calibrateServo(int idx) {
  servos[idx].servo.write(HOVER_POSITION);
}

// Reset the device.
void resetServo(int idx) {
 servos[idx].servo.write(PUSH_POSITION);
 delay(RESET_DELAY);
 servos[idx].servo.write(HOVER_POSITION);
}

// Blinks the LED so many times to indicate error.
void blinkLED(int times) {
  delay(100);
  for(int i=0; i<times; i++) {
    digitalWrite(LED_PIN, HIGH);
    delay(500);
    digitalWrite(LED_PIN, LOW);
    delay(300);
  }
}

// readCommand reads in characters from the Serial port until a newline
// is reached. The first word (delimited by a space) is returned via in the
// cmd argument and the remainder of the string is returned via the 'arg'
// argument.
void readCommand(String* cmd, String* arg) {
  const int IN_BUF_SIZE = 256;
  char in_buf[IN_BUF_SIZE];

  int n = Serial.readBytesUntil('\n', in_buf, IN_BUF_SIZE-1);
   in_buf[n] = 0;

   String inStr = String(in_buf);
   inStr.trim();
   int sep = inStr.indexOf(' ');
   if (sep == -1) {
     *cmd = String(inStr);
     cmd->trim();
     *arg = String("");
     return;
   }
   if (sep == -1) {
     sep = inStr.length();
   }
   *cmd = inStr.substring(0, sep);
   *arg = inStr.substring(sep+1, inStr.length());
}


