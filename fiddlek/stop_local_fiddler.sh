#!/bin/bash
ps aux | grep fiddler_restart.sh | awk '{print $2}' | xargs sudo kill -9
