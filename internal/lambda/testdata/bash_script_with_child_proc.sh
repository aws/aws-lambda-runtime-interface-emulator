#!/bin/sh

# Spawn one child process recursively and spin
# When parent process receives a SIGTERM, child process doesn't exit

if [ -z "$DONT_SPAWN" ]
then
    DONT_SPAWN=true ./$0 &
fi

while true
do
  sleep 1
done