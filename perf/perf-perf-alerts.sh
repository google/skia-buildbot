#!/bin/bash

cockroach sql --url postgresql://root@localhost:25000/skia?sslmode=disable -e "SELECT * FROM Alerts;" --format=csv > ~/alerts.csv