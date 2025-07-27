#!/bin/sh

echo "hello action from container " > /action/container.txt
echo "hello host from container" > /host/container.txt
echo -n "action ls: "
ls -1 /action | paste -sd ' '
echo -n "host ls: "
ls -1 /host | paste -sd ' '
echo ""
echo "exiting"
