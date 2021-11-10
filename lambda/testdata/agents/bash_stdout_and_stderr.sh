#!/usr/bin/env bash

printf "stdout line 1\n"
printf "stderr line 1\n" >&2
printf "stdout line 2\n"
printf "stderr line 2\n" >&2
printf "stdout line 3\n"
printf "stderr line 3\n" >&2
