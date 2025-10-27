# Compile Coordinator and Reducer proto definitions to Go code
#!/bin/bash
set -e
PROTOC_GEN_GO_VERSION="v1.31.0"
PROTOC_GEN_GO_GRPC_VERSION="v1.3.0"
PROTOC_VERSION="3.21.12"
GO_OUT_DIR="./pb"
mkdir -p ${GO_OUT_DIR}
# Install protoc-gen-go
if ! [ -x "$(command -v protoc-gen-go)" ]; then
    echo "Installing protoc-gen-go..."
    go install google.golang.org/protobuf/cmd/protoc-gen-go@${PROTOC_GEN_GO_VERSION}
fi
# Install protoc-gen-go-grpc
if ! [ -x "$(command -v protoc-gen-go-grpc)" ]; then
    echo "Installing protoc-gen-go-grpc..."
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@${
PROTOC_GEN_GO_GRPC_VERSION}
fi
# Download protoc if not already installed
if ! [ -x "$(command -v protoc)" ]; then
    echo "Downloading protoc..."
    PROTOC_ZIP="protoc-${PROTOC_VERSION}-linux-x86_64
.zip"
    wget
    unzip -o ${PROTOC_ZIP} -d /usr/local bin/protoc include/*
    rm -f ${PROTOC_ZIP}
fi
# Compile proto files to Go code
protoc --go_out=${GO_OUT_DIR} --go-grpc_out=${GO_OUT_DIR
} coordinator.proto reducer.proto
echo "Proto files compiled successfully to ${GO_OUT_DIR}"

    }