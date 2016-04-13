#!/usr/bin/env bash
set -e

if [[ ${#} -lt 3 ]]
then
  echo "Usage: ${0} [platform] [arch] [buildVersion]" >&2
  exit 1
fi

export GOOS=${1}
export GOARCH=${2}

BUILD_VERSION=${3}
NAME="neptune-agent"

BUILD_PATH="pkg"
BINARY_FILENAME="$NAME-$GOOS-$GOARCH"
SRC_FILE_PATH="cmd/neptuneagent/*.go"

echo -e "Building $NAME with:\n"

echo "GOOS=$GOOS"
echo "GOARCH=$GOARCH"
echo "BUILD_VERSION=$BUILD_VERSION"
echo ""

# Add .exe for Windows builds
if [[ "$GOOS" == "windows" ]]; then
  BINARY_FILENAME="$BINARY_FILENAME.exe"
  SRC_FILE_PATH="cmd/windows/*.go"
fi

mkdir -p $BUILD_PATH
go build -v -a -o $BUILD_PATH/$BINARY_FILENAME $SRC_FILE_PATH

chmod +x $BUILD_PATH/$BINARY_FILENAME

echo -e "\nDone: \033[33m$BUILD_PATH/$BINARY_FILENAME\033[0m 💪"
