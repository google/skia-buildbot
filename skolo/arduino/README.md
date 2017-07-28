Switching servos with Arduino
=============================

cmd_runner contains a simple Arduino program that
listens to commands sent over the serial/USB
connection. The main command is 'reset' which will
initate a reset procedure by switcing a servo.
See the source of cmd_runner for details.

Building
========

Install the Arduino IDE on Linux via

$ apt-get intall arduino

and then run 'arduino' from the command line.
This will open the Arduino IDE where you can open the
cmd_runner program.

Attach the Arduino board and upload the program.
See https://www.arduino.cc/ for more documentation.


Downloading and Installing Libraries
=====================

The Servo shield requires the [Adafruit PWM Servo Driver Library](https://github.com/adafruit/Adafruit-PWM-Servo-Driver-Library)

Download the release from GitHub, extract it and rename the folder to
Adafruit_PWMServoDriver.  Then, place it in the libraries folder of your Arduino IDE
installation. Restart the IDE.