#!/usr/bin/env bash
set -e

if [[ ${#} -lt 3 ]]
then
  echo "Usage: ${0} [platform] [arch] [buildVersion]" >&2
  exit 1
fi

export GOOS=${1}
GOARCH=${2}
if [[ "$GOARCH" == "ARMv7" ]]; then
    export GOARM=7
    ARCH="armv7"
    GOARCH=""
else
    # For ARMv8 and later versions, set GOARCH instead of GOARM as per instructions at
    # https://github.com/golang/go/wiki/GoArm
    export GOARCH="arm64"
    ARCH="armv8"
fi

BUILD_VERSION=${3}
NAME="neptune-agent"

BUILD_PATH="pkg"
BINARY_FILENAME="$NAME-$GOOS-$ARCH"
SRC_FILE_PATH="cmd/agent/*.go"

echo -e "Building $NAME with:\n"

echo "GOOS=$GOOS"
echo "GOARCH=$GOARCH"
echo "GOARM=$GOARM"
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
