# This Makefile is meant to be used by people that do not usually work
# with Go source code. If you know what GOPATH is then you probably
# don't need to bother with make.

.PHONY: g420 android ios g420-cross evm all test clean
.PHONY: g420-linux g420-linux-386 g420-linux-amd64 g420-linux-mips64 g420-linux-mips64le
.PHONY: g420-linux-arm g420-linux-arm-5 g420-linux-arm-6 g420-linux-arm-7 g420-linux-arm64
.PHONY: g420-darwin g420-darwin-386 g420-darwin-amd64
.PHONY: g420-windows g420-windows-386 g420-windows-amd64

GOBIN = ./build/bin
GO ?= latest
GORUN = env GO111MODULE=on go run

g420:
	$(GORUN) build/ci.go install ./cmd/g420
	@echo "Done building."
	@echo "Run \"$(GOBIN)/g420\" to launch g420."

all:
	$(GORUN) build/ci.go install

android:
	$(GORUN) build/ci.go aar --local
	@echo "Done building."
	@echo "Import \"$(GOBIN)/g420.aar\" to use the library."

ios:
	$(GORUN) build/ci.go xcode --local
	@echo "Done building."
	@echo "Import \"$(GOBIN)/G420.framework\" to use the library."

test: all
	$(GORUN) build/ci.go test

lint: ## Run linters.
	$(GORUN) build/ci.go lint

clean:
	env GO111MODULE=on go clean -cache
	rm -fr build/_workspace/pkg/ $(GOBIN)/*

# The devtools target installs tools required for 'go generate'.
# You need to put $GOBIN (or $GOPATH/bin) in your PATH to use 'go generate'.

devtools:
	env GOBIN= go get -u golang.org/x/tools/cmd/stringer
	env GOBIN= go get -u github.com/kevinburke/go-bindata/go-bindata
	env GOBIN= go get -u github.com/fjl/gencodec
	env GOBIN= go get -u github.com/golang/protobuf/protoc-gen-go
	env GOBIN= go install ./cmd/abigen
	@type "npm" 2> /dev/null || echo 'Please install node.js and npm'
	@type "solc" 2> /dev/null || echo 'Please install solc'
	@type "protoc" 2> /dev/null || echo 'Please install protoc'

# Cross Compilation Targets (xgo)

g420-cross: g420-linux g420-darwin g420-windows g420-android g420-ios
	@echo "Full cross compilation done:"
	@ls -ld $(GOBIN)/g420-*

g420-linux: g420-linux-386 g420-linux-amd64 g420-linux-arm g420-linux-mips64 g420-linux-mips64le
	@echo "Linux cross compilation done:"
	@ls -ld $(GOBIN)/g420-linux-*

g420-linux-386:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=linux/386 -v ./cmd/g420
	@echo "Linux 386 cross compilation done:"
	@ls -ld $(GOBIN)/g420-linux-* | grep 386

g420-linux-amd64:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=linux/amd64 -v ./cmd/g420
	@echo "Linux amd64 cross compilation done:"
	@ls -ld $(GOBIN)/g420-linux-* | grep amd64

g420-linux-arm: g420-linux-arm-5 g420-linux-arm-6 g420-linux-arm-7 g420-linux-arm64
	@echo "Linux ARM cross compilation done:"
	@ls -ld $(GOBIN)/g420-linux-* | grep arm

g420-linux-arm-5:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=linux/arm-5 -v ./cmd/g420
	@echo "Linux ARMv5 cross compilation done:"
	@ls -ld $(GOBIN)/g420-linux-* | grep arm-5

g420-linux-arm-6:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=linux/arm-6 -v ./cmd/g420
	@echo "Linux ARMv6 cross compilation done:"
	@ls -ld $(GOBIN)/g420-linux-* | grep arm-6

g420-linux-arm-7:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=linux/arm-7 -v ./cmd/g420
	@echo "Linux ARMv7 cross compilation done:"
	@ls -ld $(GOBIN)/g420-linux-* | grep arm-7

g420-linux-arm64:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=linux/arm64 -v ./cmd/g420
	@echo "Linux ARM64 cross compilation done:"
	@ls -ld $(GOBIN)/g420-linux-* | grep arm64

g420-linux-mips:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=linux/mips --ldflags '-extldflags "-static"' -v ./cmd/g420
	@echo "Linux MIPS cross compilation done:"
	@ls -ld $(GOBIN)/g420-linux-* | grep mips

g420-linux-mipsle:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=linux/mipsle --ldflags '-extldflags "-static"' -v ./cmd/g420
	@echo "Linux MIPSle cross compilation done:"
	@ls -ld $(GOBIN)/g420-linux-* | grep mipsle

g420-linux-mips64:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=linux/mips64 --ldflags '-extldflags "-static"' -v ./cmd/g420
	@echo "Linux MIPS64 cross compilation done:"
	@ls -ld $(GOBIN)/g420-linux-* | grep mips64

g420-linux-mips64le:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=linux/mips64le --ldflags '-extldflags "-static"' -v ./cmd/g420
	@echo "Linux MIPS64le cross compilation done:"
	@ls -ld $(GOBIN)/g420-linux-* | grep mips64le

g420-darwin: g420-darwin-386 g420-darwin-amd64
	@echo "Darwin cross compilation done:"
	@ls -ld $(GOBIN)/g420-darwin-*

g420-darwin-386:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=darwin/386 -v ./cmd/g420
	@echo "Darwin 386 cross compilation done:"
	@ls -ld $(GOBIN)/g420-darwin-* | grep 386

g420-darwin-amd64:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=darwin/amd64 -v ./cmd/g420
	@echo "Darwin amd64 cross compilation done:"
	@ls -ld $(GOBIN)/g420-darwin-* | grep amd64

g420-windows: g420-windows-386 g420-windows-amd64
	@echo "Windows cross compilation done:"
	@ls -ld $(GOBIN)/g420-windows-*

g420-windows-386:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=windows/386 -v ./cmd/g420
	@echo "Windows 386 cross compilation done:"
	@ls -ld $(GOBIN)/g420-windows-* | grep 386

g420-windows-amd64:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=windows/amd64 -v ./cmd/g420
	@echo "Windows amd64 cross compilation done:"
	@ls -ld $(GOBIN)/g420-windows-* | grep amd64
