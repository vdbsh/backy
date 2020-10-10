#!/bin/bash
# Install script example for MacOS Launchd agent
#
# More info at:
# https://en.wikipedia.org/wiki/Launchd
# https://developer.apple.com/library/archive/documentation/MacOSX/Conceptual/BPSystemStartup/Chapters/ScheduledJobs.html#//apple_ref/doc/uid/10000172i-CH1-SW2

set -e

BACKY_BIN=$HOME/backy/backy
AGENT_TEMPLATE_FILE=com.backy.backup.plist
TASK_FILE=$PWD/example_task.json
AGENT_NAME=com.backy.example.backup.hourly

LAUNCH_AGENTS_DIR=$HOME/Library/LaunchAgents
AGENT_FILE=$LAUNCH_AGENTS_DIR/$AGENT_NAME.plist

if [[ "$OSTYPE" == "darwin"* ]]; then
    if [ -f $AGENT_FILE ]; then
        launchctl unload -w $AGENT_FILE 2> /dev/null
        rm -f $AGENT_FILE
    elif [ ! -d $LAUNCH_AGENTS_DIR ]; then
        mkdir $LAUNCH_AGENTS_DIR
    fi

    cp $AGENT_TEMPLATE_FILE $AGENT_FILE

    sed -i ""  "s,AGENT_NAME,$AGENT_NAME,g" $AGENT_FILE
    sed -i ""  "s,BACKY_BIN,$BACKY_BIN,g" $AGENT_FILE
    sed -i ""  "s,TASK_FILE,$TASK_FILE,g" $AGENT_FILE

    launchctl load -w $AGENT_FILE

    if launchctl list | grep $AGENT_NAME ; then
        echo "Installation complete!"
        cat $AGENT_FILE
    else
        echo "Something went wrong..."
    fi
else
    echo "This script is intended to run on macOS" 
    echo "Please use cron if your OS is different"
    exit 1
fi
