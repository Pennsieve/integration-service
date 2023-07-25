#! /bin/bash
set -e

echo ""
echo "**********************************"
echo "*   Testing Integration Service Lambda   *"
echo "**********************************"
echo ""
cd ./lambda/service; \
  go test -v ./... ;
