#!/bin/bash
log_dir=".logs/"
mkdir -p $log_dir
log_name=$(date '+%Y-%m-%d:%H-%M-%S')
nohup twitter > "$log_dir$log_name.log" 2>&1 &
