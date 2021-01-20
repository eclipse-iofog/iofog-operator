#!/bin/sh

######## These variables MUST be updated by the user / automation task

# Specify a non-existent ephemeral namespace for testing purposes
export NAMESPACE="<<NAMESPACE>>"

######################################################################

echo ""
echo "----- CONFIG -----"
echo ""
echo "${!NAMESPACE*}: " "$NAMESPACE"
echo ""
echo "------------------"
echo ""
