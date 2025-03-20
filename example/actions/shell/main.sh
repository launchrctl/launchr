#!/bin/sh

echo
# Print the total count of arguments
echo "Total number of arguments passed to the script: $#"
count=1
for arg in "$@"
do
    if [ -z "$arg" ]; then
        echo "- Argument $count: \"\""
    else
        echo "- Argument $count: $arg"
    fi
    count=$((count + 1))
done
