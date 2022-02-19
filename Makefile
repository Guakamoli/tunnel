export GO111MODULE=on
LDFLAGS := -s -w

os-archs=darwin:amd64 darwin:arm64 linux:386 linux:amd64 linux:arm linux:arm64 windows:386 windows:amd64

all: fmt build

fmt:
	go fmt ./...

build: clean tunnel

tunnel:
	env CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o ./bin/tunnel .

clean:
	rm -f ./bin/tunnel

app:
	@$(foreach n, $(os-archs),\
		os=$(shell echo "$(n)" | cut -d : -f 1);\
		arch=$(shell echo "$(n)" | cut -d : -f 2);\
		gomips=$(shell echo "$(n)" | cut -d : -f 3);\
		target_suffix=$${os}_$${arch};\
		echo "Build $${os}-$${arch}...";\
		env CGO_ENABLED=0 GOOS=$${os} GOARCH=$${arch} GOMIPS=$${gomips} go build -trimpath -ldflags "$(LDFLAGS)" -o ./build/tunnel_$${target_suffix} .;\
		echo "Build $${os}-$${arch} done";\
	)
	@mv ./build/tunnel_windows_386 ./build/tunnel_windows_386.exe
	@mv ./build/tunnel_windows_amd64 ./build/tunnel_windows_amd64.exe
