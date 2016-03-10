#!/usr/bin/env bash
set -e

if [[ ${#} -lt 2 ]]
then
  echo "Usage: ${0} [file] [version]" >&2
  exit 1
fi

function info {
  echo -e "\033[35m$1\033[0m"
}

BINARY_PATH=${1}

BASE_DIRECTORY=`pwd`
CONF_DIRECTORY=$BASE_DIRECTORY/config
CERT_DIRECTORY=$BASE_DIRECTORY/cert
TMP_DIRECTORY=$BASE_DIRECTORY/tmp
RELEASE_DIRECTORY=$BASE_DIRECTORY/releases

# Find the base name of the binary without the extension (if there is one)
RELEASE_NAME=$(basename "$BINARY_PATH")
RELEASE_NAME="${RELEASE_NAME%.*}-$2"

# Where we will construct the release
TMP_RELEASE_DIRECTORY=$TMP_DIRECTORY/$RELEASE_NAME

# Ensure the tmp release directory exists
rm -rf $TMP_RELEASE_DIRECTORY
mkdir -p $TMP_RELEASE_DIRECTORY

# We need to use .zip for windows builds
if [[ "$BINARY_PATH" == *"windows"* ]]; then
  RELEASE_FILE_NAME="$RELEASE_NAME.zip"

  info "Copying binary"
  cp $BINARY_PATH $TMP_RELEASE_DIRECTORY/neptune-agent.exe

  info "Copying config file"
  cp $CONF_DIRECTORY/neptune-agent.json $TMP_RELEASE_DIRECTORY

  info "Copying the certificate file"
  cp $CERT_DIRECTORY/neptuneio.crt $TMP_RELEASE_DIRECTORY

  info "Zipping up the files"
  cd $TMP_RELEASE_DIRECTORY
  zip -X -r "../$RELEASE_FILE_NAME" *
else
  RELEASE_FILE_NAME="$RELEASE_NAME.tar.gz"

  info "Copying binary"
  cp $BINARY_PATH $TMP_RELEASE_DIRECTORY/neptune-agent
  chmod +x $TMP_RELEASE_DIRECTORY/neptune-agent

  info "Copying config file"
  cp $CONF_DIRECTORY/neptune-agent.json $TMP_RELEASE_DIRECTORY

  info "Copying the certificate file"
  cp $CERT_DIRECTORY/neptuneio.crt $TMP_RELEASE_DIRECTORY

  info "Tarring up the files"
  cd $TMP_RELEASE_DIRECTORY
  tar cfvz ../$RELEASE_FILE_NAME .
fi

mkdir -p $RELEASE_DIRECTORY
mv $TMP_DIRECTORY/$RELEASE_FILE_NAME $RELEASE_DIRECTORY/

echo -e "👏 Created release \033[33m$RELEASE_DIRECTORY/$RELEASE_FILE_NAME\033[0m"
